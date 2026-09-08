// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {IFieldHasher} from "./IFieldHasher.sol";
import {IZkVerifier} from "./IZkVerifier.sol";

/// @notice M1 main ledger.  Audit decryption and graph traversal remain off-chain.
contract ZkDPPV2Ledger {
    uint256 public constant TREE_DEPTH = 32;
    uint256 private constant FIELD = 52435875175126190479447740508185965837690552500527637822603658699938581184513;
    uint256 public constant POLICY_REF_TAG = 51780253709110118800953810799850621917828020824397005216617999075326973004822;
    uint8 public constant NOTE = 1;
    uint8 public constant VOUCHER = 2;
    uint8 public constant CLAIM = 3;
    uint8 public constant ACTIVE = 0;
    uint8 public constant FROZEN = 1;
    uint8 public constant REVOKED = 2;
    uint8 public constant PROCESS = 6;
    uint8 public constant ISSUE = 8;

    struct Tree {
        mapping(uint256 => bool) acceptedRoots;
        mapping(uint256 => uint256) nodes;
        uint256[TREE_DEPTH + 1] zeroes;
        uint256 leafCount;
        uint256 currentRoot;
    }
    struct ObjectRef { uint8 objectType; uint256 rawId; }
    struct AuditCipher { uint256 r1X; uint256 r1Y; uint256[] encryptedParents; uint256[] encryptedOutputNfs; }
    struct AuditRecord {
        uint8 eventKind;
        uint256 policyRef;
        ObjectRef[] outputRefs;
        uint256 r1X;
        uint256 r1Y;
        uint256[] encryptedParents;
        uint256[] encryptedOutputNfs;
    }
    struct PolicyAuthority { address account; bool enabled; }
    struct PolicyFamily { uint8 eventKind; bool exists; }
    struct PolicyReservation { uint64 authorityId; uint64 policyId; uint64 version; uint8 eventKind; bool exists; }
    struct PolicyRecord {
        uint64 authorityId;
        uint64 policyId;
        uint64 version;
        uint8 eventKind;
        uint8 inputArity;
        uint8 outputArity;
        bytes32 vkHash;
        address verifierRef;
        bool enabled;
    }

    address public immutable admin;
    address public immutable statusAuthority;
    IFieldHasher public immutable fieldHasher;
    IZkVerifier[7] public fixedEventVerifiers;
    Tree private noteTree;
    Tree private voucherTree;

    mapping(address => bool) public entryIssuers;
    mapping(uint256 => bool) public commitments;
    mapping(uint256 => bool) public voucherCommitments;
    mapping(uint8 => mapping(uint256 => uint256)) public producerOf;
    mapping(uint256 => uint256) public noteSpentIn;
    mapping(uint256 => uint256) public voucherSpentIn;
    mapping(uint256 => uint8) public noteStatusByNf;
    mapping(uint256 => uint8) public voucherStatusByNf;
    mapping(uint256 => bool) public claimRegistered;
    uint256 public nextAuditRecordId = 1;
    mapping(uint256 => AuditRecord) private auditRecords;

    uint64 public nextAuthorityId = 1;
    mapping(address => uint64) public authorityIdOf;
    mapping(uint64 => PolicyAuthority) public policyAuthorities;
    mapping(uint64 => uint64) public nextPolicyId;
    mapping(uint64 => mapping(uint64 => uint64)) public nextPolicyVersion;
    mapping(uint64 => mapping(uint64 => PolicyFamily)) public policyFamilies;
    mapping(uint256 => PolicyReservation) public policyReservations;
    mapping(uint256 => PolicyRecord) public policyRecords;
    mapping(uint256 => mapping(uint256 => bool)) public policyGrants;

    event EntryIssuerUpdated(address indexed account, bool allowed);
    event NoteAppended(uint256 indexed cm, uint256 index, uint256 root);
    event VoucherAppended(uint256 indexed rv, uint256 index, uint256 root);
    event AuditRecorded(uint256 indexed auditRecordId, uint8 indexed eventKind);
    event ClaimIssued(uint256 indexed h, uint256 indexed issuePolicyRef, uint256 indexed auditRecordId);
    event NullifierStatusChanged(uint8 indexed objectType, uint256 indexed spendValue, uint8 oldStatus, uint8 newStatus);
    event PolicyAuthorityRegistered(uint64 indexed authorityId, address indexed account);
    event PolicyReserved(uint256 indexed policyRef, uint64 indexed authorityId, uint64 policyId, uint64 version, uint8 eventKind);
    event PolicyRegistered(uint256 indexed policyRef, address indexed verifierRef, bytes32 vkHash);
    event PolicyGrantUpdated(uint256 indexed policyRef, uint256 indexed policyScopeRef, bool allowed);
    event PolicyDisabled(uint256 indexed policyRef);

    error Unauthorized(); error InvalidAccount(); error InvalidProof(); error InvalidField(); error InvalidRoot();
    error InvalidShape(); error InvalidState(); error InvalidObject(); error DuplicateObject(); error AlreadySpent();
    error InvalidDeadline(); error UnknownPolicy(); error InvalidPolicy(); error MissingGrant(); error UnknownRecord();

    constructor(address[7] memory verifiers, address hasher, address authority) {
        if (hasher.code.length == 0 || authority == address(0)) revert InvalidAccount();
        for (uint256 i; i < 7; i++) {
            if (verifiers[i].code.length == 0) revert InvalidAccount();
            fixedEventVerifiers[i] = IZkVerifier(verifiers[i]);
        }
        admin = msg.sender;
        statusAuthority = authority;
        fieldHasher = IFieldHasher(hasher);
        _initTree(noteTree);
        _initTree(voucherTree);
    }

    function setEntryIssuer(address account, bool allowed) external {
        if (msg.sender != admin) revert Unauthorized();
        if (account == address(0)) revert InvalidAccount();
        entryIssuers[account] = allowed;
        emit EntryIssuerUpdated(account, allowed);
    }

    function entry(bytes calldata proof, uint256 cm, AuditCipher calldata auditCipher) external returns (uint256) {
        if (!entryIssuers[msg.sender]) revert Unauthorized();
        return _recordFixed(0, proof, _one(cm), new uint256[](0), _refs(NOTE, cm), auditCipher);
    }

    function transfer(bytes calldata proof, uint256 root, uint256 nf, uint256 rvNew, uint256 cmChange, uint256 D, AuditCipher calldata auditCipher) external returns (uint256) {
        if (!noteTree.acceptedRoots[root]) revert InvalidRoot();
        _active(NOTE, nf);
        if (block.number >= D) revert InvalidDeadline();
        uint256[] memory p = new uint256[](5);
        p[0]=root; p[1]=nf; p[2]=rvNew; p[3]=cmChange; p[4]=D;
        return _recordFixed(1, proof, p, _one(nf), _refs2(VOUCHER, rvNew, NOTE, cmChange), auditCipher);
    }

    function proceed(bytes calldata proof, uint256 root, uint256 rvnf, uint256 cmReceiver, AuditCipher calldata auditCipher) external returns (uint256) {
        if (!voucherTree.acceptedRoots[root]) revert InvalidRoot();
        _active(VOUCHER, rvnf);
        return _recordFixed(2, proof, _three(root, rvnf, cmReceiver), _one(rvnf), _refs(NOTE, cmReceiver), auditCipher);
    }

    function recall(bytes calldata proof, uint256 root, uint256 rvnf, uint256 cmReturn, uint256 D, AuditCipher calldata auditCipher) external returns (uint256) {
        if (!voucherTree.acceptedRoots[root]) revert InvalidRoot();
        _active(VOUCHER, rvnf);
        if (block.number > D) revert InvalidDeadline();
        return _recordFixed(3, proof, _four(root, rvnf, cmReturn, D), _one(rvnf), _refs(NOTE, cmReturn), auditCipher);
    }

    function merge(bytes calldata proof, uint256 root, uint256 nf1, uint256 nf2, uint256 cmOut, AuditCipher calldata auditCipher) external returns (uint256) {
        if (!noteTree.acceptedRoots[root]) revert InvalidRoot();
        if (nf1 == nf2) revert InvalidShape();
        _active(NOTE, nf1); _active(NOTE, nf2);
        uint256[] memory spends = new uint256[](2); spends[0]=nf1; spends[1]=nf2;
        return _recordFixed(4, proof, _four(root,nf1,nf2,cmOut), spends, _refs(NOTE,cmOut), auditCipher);
    }

    function split(bytes calldata proof, uint256 root, uint256 nf, uint256 cmOut1, uint256 cmOut2, AuditCipher calldata auditCipher) external returns (uint256) {
        if (!noteTree.acceptedRoots[root]) revert InvalidRoot();
        _active(NOTE, nf);
        return _recordFixed(5, proof, _four(root,nf,cmOut1,cmOut2), _one(nf), _refs2(NOTE,cmOut1,NOTE,cmOut2), auditCipher);
    }

    function process(bytes calldata proof, uint256 policyRef, uint256 policyScopeRef, uint256 root, uint256[3] calldata nfs, uint256[2] calldata outputs, AuditCipher calldata auditCipher) external returns (uint256) {
        PolicyRecord memory policy = _policy(policyRef, PROCESS, 3, 2);
        if (!policyGrants[policyRef][policyScopeRef]) revert MissingGrant();
        if (!noteTree.acceptedRoots[root]) revert InvalidRoot();
        uint256[] memory p = new uint256[](8); p[0]=policyRef; p[1]=policyScopeRef; p[2]=root;
        uint256[] memory spends = new uint256[](3);
        for (uint256 i; i<3; i++) {
            _active(NOTE,nfs[i]); p[3+i]=nfs[i]; spends[i]=nfs[i];
            for (uint256 j; j<i; j++) if (nfs[i]==nfs[j]) revert InvalidShape();
        }
        p[6]=outputs[0]; p[7]=outputs[1];
        return _recordPolicy(6, IZkVerifier(policy.verifierRef), proof, p, spends, _refs2(NOTE,outputs[0],NOTE,outputs[1]), auditCipher, policyRef);
    }

    function exit(bytes calldata proof, uint256 root, uint256 nf, AuditCipher calldata auditCipher) external returns (uint256) {
        if (!noteTree.acceptedRoots[root]) revert InvalidRoot();
        _active(NOTE,nf);
        return _recordFixed(7,proof,_two(root,nf),_one(nf),new ObjectRef[](0),auditCipher);
    }

    function issue(bytes calldata proof, uint256 issuePolicyRef, uint256 root, uint256 nf, uint256 h, AuditCipher calldata auditCipher) external returns (uint256 aid) {
        PolicyRecord memory policy = _policy(issuePolicyRef, ISSUE, 1, 1);
        if (!noteTree.acceptedRoots[root]) revert InvalidRoot();
        _active(NOTE,nf);
        if (claimRegistered[h]) revert DuplicateObject();
        uint256[] memory p = _four(issuePolicyRef,root,nf,h);
        aid = _recordPolicy(8,IZkVerifier(policy.verifierRef),proof,p,_one(nf),_refs(CLAIM,h),auditCipher,issuePolicyRef);
        claimRegistered[h] = true;
        emit ClaimIssued(h,issuePolicyRef,aid);
    }

    function _recordFixed(uint8 kind, bytes calldata proof, uint256[] memory base, uint256[] memory spends, ObjectRef[] memory outputs, AuditCipher calldata cipher) private returns (uint256) {
        uint256 verifierIndex = kind == 7 ? 6 : kind;
        return _recordPolicy(kind,fixedEventVerifiers[verifierIndex],proof,base,spends,outputs,cipher,0);
    }

    function _recordPolicy(uint8 kind, IZkVerifier verifier, bytes calldata proof, uint256[] memory base, uint256[] memory spends, ObjectRef[] memory outputs, AuditCipher calldata cipher, uint256 policyRef) private returns (uint256 aid) {
        uint256 parents = kind==0 ? 0 : kind==4 ? 2 : kind==6 ? 3 : 1;
        uint256 future = kind==0||kind==2||kind==3||kind==4 ? 1 : kind==1||kind==5||kind==6 ? 2 : 0;
        if (cipher.encryptedParents.length!=parents || cipher.encryptedOutputNfs.length!=future || outputs.length!=(kind==7?0:kind==8?1:future)) revert InvalidShape();
        _fresh(outputs);
        uint256[] memory inputs = new uint256[](base.length+2+parents+future);
        for(uint256 i;i<base.length;i++) inputs[i]=base[i];
        inputs[base.length]=cipher.r1X; inputs[base.length+1]=cipher.r1Y;
        for(uint256 i;i<parents;i++) inputs[base.length+2+i]=cipher.encryptedParents[i];
        for(uint256 i;i<future;i++) inputs[base.length+2+parents+i]=cipher.encryptedOutputNfs[i];
        for(uint256 i;i<inputs.length;i++) if(inputs[i]>=FIELD) revert InvalidField();
        _verify(verifier,proof,inputs);
        aid=nextAuditRecordId++;
        bool voucherInput=kind==2||kind==3;
        for(uint256 i;i<spends.length;i++) {
            if(voucherInput) voucherSpentIn[spends[i]]=aid; else noteSpentIn[spends[i]]=aid;
        }
        AuditRecord storage rec=auditRecords[aid];
        rec.eventKind=kind; rec.policyRef=policyRef; rec.r1X=cipher.r1X; rec.r1Y=cipher.r1Y;
        rec.encryptedParents=cipher.encryptedParents; rec.encryptedOutputNfs=cipher.encryptedOutputNfs;
        for(uint256 i;i<outputs.length;i++) {
            ObjectRef memory ref=outputs[i]; rec.outputRefs.push(ref); producerOf[ref.objectType][ref.rawId]=aid;
            if(ref.objectType==NOTE) { commitments[ref.rawId]=true; _append(noteTree,ref.rawId,false); }
            else if(ref.objectType==VOUCHER) { voucherCommitments[ref.rawId]=true; _append(voucherTree,ref.rawId,true); }
        }
        emit AuditRecorded(aid,kind);
    }

    function setStatus(uint8 objectType, uint256 spendValue, uint8 newStatus) external {
        if(msg.sender!=statusAuthority) revert Unauthorized();
        if(objectType!=NOTE && objectType!=VOUCHER) revert InvalidObject();
        if(spendValue>=FIELD) revert InvalidField();
        if((objectType==NOTE?noteSpentIn[spendValue]:voucherSpentIn[spendValue])!=0) revert AlreadySpent();
        uint8 oldStatus=objectType==NOTE?noteStatusByNf[spendValue]:voucherStatusByNf[spendValue];
        if(!((oldStatus==ACTIVE&&newStatus==FROZEN)||(oldStatus==FROZEN&&(newStatus==ACTIVE||newStatus==REVOKED)))) revert InvalidState();
        if(objectType==NOTE) noteStatusByNf[spendValue]=newStatus; else voucherStatusByNf[spendValue]=newStatus;
        emit NullifierStatusChanged(objectType,spendValue,oldStatus,newStatus);
    }

    function registerPolicyAuthority(address account) external returns(uint64 id) {
        if(msg.sender!=admin) revert Unauthorized();
        if(account==address(0)||authorityIdOf[account]!=0) revert InvalidAccount();
        id=nextAuthorityId++; authorityIdOf[account]=id; policyAuthorities[id]=PolicyAuthority(account,true); nextPolicyId[id]=1;
        emit PolicyAuthorityRegistered(id,account);
    }
    function reservePolicy(uint8 eventKind) external returns(uint256 ref) {
        uint64 a=_authority(msg.sender); if(eventKind!=PROCESS&&eventKind!=ISSUE) revert InvalidPolicy();
        uint64 id=nextPolicyId[a]++; policyFamilies[a][id]=PolicyFamily(eventKind,true); nextPolicyVersion[a][id]=2;
        ref=_computePolicyRef(eventKind,a,id,1); policyReservations[ref]=PolicyReservation(a,id,1,eventKind,true);
        emit PolicyReserved(ref,a,id,1,eventKind);
    }
    function reservePolicyVersion(uint64 policyId) external returns(uint256 ref) {
        uint64 a=_authority(msg.sender); PolicyFamily memory family=policyFamilies[a][policyId]; if(!family.exists) revert InvalidPolicy();
        uint64 version=nextPolicyVersion[a][policyId]; uint256 previous=_computePolicyRef(family.eventKind,a,policyId,version-1);
        if(policyRecords[previous].authorityId==0) revert InvalidPolicy();
        nextPolicyVersion[a][policyId]=version+1; ref=_computePolicyRef(family.eventKind,a,policyId,version);
        policyReservations[ref]=PolicyReservation(a,policyId,version,family.eventKind,true);
        emit PolicyReserved(ref,a,policyId,version,family.eventKind);
    }
    function registerPolicy(uint256 ref,uint8 inputArity,uint8 outputArity,bytes32 vkHash,address verifierRef) external {
        uint64 a=_authority(msg.sender); PolicyReservation memory r=policyReservations[ref];
        if(!r.exists||r.authorityId!=a||policyRecords[ref].authorityId!=0||verifierRef.code.length==0||vkHash==0) revert InvalidPolicy();
        if(r.eventKind==PROCESS&&(inputArity!=3||outputArity!=2)||r.eventKind==ISSUE&&(inputArity!=1||outputArity!=1)) revert InvalidPolicy();
        policyRecords[ref]=PolicyRecord(a,r.policyId,r.version,r.eventKind,inputArity,outputArity,vkHash,verifierRef,true);
        emit PolicyRegistered(ref,verifierRef,vkHash);
    }
    function setPolicyGrant(uint256 ref,uint256 scope,bool allowed) external {
        PolicyRecord memory p=policyRecords[ref]; if(p.authorityId==0||_authority(msg.sender)!=p.authorityId||p.eventKind!=PROCESS||scope==0) revert InvalidPolicy();
        policyGrants[ref][scope]=allowed; emit PolicyGrantUpdated(ref,scope,allowed);
    }
    function disablePolicy(uint256 ref) external {
        PolicyRecord storage p=policyRecords[ref]; if(p.authorityId==0||_authority(msg.sender)!=p.authorityId||!p.enabled) revert InvalidPolicy();
        p.enabled=false; emit PolicyDisabled(ref);
    }

    function getAuditRecord(uint256 id) external view returns(AuditRecord memory) { if(id==0||id>=nextAuditRecordId) revert UnknownRecord(); return auditRecords[id]; }
    function currentNoteRoot() external view returns(uint256){return noteTree.currentRoot;}
    function currentVoucherRoot() external view returns(uint256){return voucherTree.currentRoot;}
    function acceptedNoteRoot(uint256 root) external view returns(bool){return noteTree.acceptedRoots[root];}
    function acceptedVoucherRoot(uint256 root) external view returns(bool){return voucherTree.acceptedRoots[root];}
    function noteLeafCount() external view returns(uint256){return noteTree.leafCount;}
    function voucherLeafCount() external view returns(uint256){return voucherTree.leafCount;}
    function noteLeaf(uint256 index) external view returns(uint256){if(index>=noteTree.leafCount)revert InvalidObject();return noteTree.nodes[_nodeIndex(0,index)];}
    function voucherLeaf(uint256 index) external view returns(uint256){if(index>=voucherTree.leafCount)revert InvalidObject();return voucherTree.nodes[_nodeIndex(0,index)];}
    function computePolicyRef(uint8 kind,uint64 a,uint64 p,uint64 v) external view returns(uint256){return _computePolicyRef(kind,a,p,v);}

    function _policy(uint256 ref,uint8 kind,uint8 inputs,uint8 outputs) private view returns(PolicyRecord memory p) {
        p=policyRecords[ref]; if(p.authorityId==0) revert UnknownPolicy();
        if(!p.enabled||p.eventKind!=kind||p.inputArity!=inputs||p.outputArity!=outputs) revert InvalidPolicy();
    }
    function _authority(address account) private view returns(uint64 id){id=authorityIdOf[account];if(id==0||!policyAuthorities[id].enabled)revert Unauthorized();}
    function _computePolicyRef(uint8 kind,uint64 a,uint64 p,uint64 v) private view returns(uint256 state){state=fieldHasher.compress(0,POLICY_REF_TAG);state=fieldHasher.compress(state,kind);state=fieldHasher.compress(state,a);state=fieldHasher.compress(state,p);state=fieldHasher.compress(state,v);}
    function _active(uint8 t,uint256 s) private view {if(s>=FIELD)revert InvalidField();if(t==NOTE){if(noteSpentIn[s]!=0)revert AlreadySpent();if(noteStatusByNf[s]!=ACTIVE)revert InvalidState();}else{if(voucherSpentIn[s]!=0)revert AlreadySpent();if(voucherStatusByNf[s]!=ACTIVE)revert InvalidState();}}
    function _fresh(ObjectRef[] memory refs) private view {for(uint256 i;i<refs.length;i++){if(refs[i].rawId>=FIELD)revert InvalidField();if(refs[i].objectType==NOTE&&commitments[refs[i].rawId]||refs[i].objectType==VOUCHER&&voucherCommitments[refs[i].rawId]||refs[i].objectType==CLAIM&&claimRegistered[refs[i].rawId])revert DuplicateObject();for(uint256 j;j<i;j++)if(refs[i].objectType==refs[j].objectType&&refs[i].rawId==refs[j].rawId)revert DuplicateObject();}}
    function _verify(IZkVerifier verifier,bytes calldata proof,uint256[] memory inputs) private view {try verifier.Verify(proof,inputs) returns(bool valid){if(!valid)revert InvalidProof();}catch{revert InvalidProof();}}
    function _initTree(Tree storage tree) private {for(uint256 i;i<TREE_DEPTH;i++)tree.zeroes[i+1]=fieldHasher.compress(tree.zeroes[i],tree.zeroes[i]);tree.currentRoot=tree.zeroes[TREE_DEPTH];tree.acceptedRoots[tree.currentRoot]=true;}
    function _append(Tree storage tree,uint256 leaf,bool isVoucher) private {if(tree.leafCount>=uint256(1)<<TREE_DEPTH)revert InvalidState();uint256 index=tree.leafCount;uint256 pos=index;uint256 current=leaf;tree.nodes[_nodeIndex(0,pos)]=current;for(uint256 level;level<TREE_DEPTH;level++){uint256 sibling=tree.nodes[_nodeIndex(level,pos^1)];if(sibling==0)sibling=tree.zeroes[level];current=(pos&1)==0?fieldHasher.compress(current,sibling):fieldHasher.compress(sibling,current);pos>>=1;tree.nodes[_nodeIndex(level+1,pos)]=current;}tree.leafCount=index+1;tree.currentRoot=current;tree.acceptedRoots[current]=true;if(isVoucher)emit VoucherAppended(leaf,index,current);else emit NoteAppended(leaf,index,current);}
    function _nodeIndex(uint256 level,uint256 position) private pure returns(uint256){return(uint256(1)<<(TREE_DEPTH-level))-1+position;}
    function _refs(uint8 t,uint256 id) private pure returns(ObjectRef[] memory r){r=new ObjectRef[](1);r[0]=ObjectRef(t,id);}
    function _refs2(uint8 t1,uint256 id1,uint8 t2,uint256 id2) private pure returns(ObjectRef[] memory r){r=new ObjectRef[](2);r[0]=ObjectRef(t1,id1);r[1]=ObjectRef(t2,id2);}
    function _one(uint256 a) private pure returns(uint256[] memory v){v=new uint256[](1);v[0]=a;}
    function _two(uint256 a,uint256 b) private pure returns(uint256[] memory v){v=new uint256[](2);v[0]=a;v[1]=b;}
    function _three(uint256 a,uint256 b,uint256 c) private pure returns(uint256[] memory v){v=new uint256[](3);v[0]=a;v[1]=b;v[2]=c;}
    function _four(uint256 a,uint256 b,uint256 c,uint256 d) private pure returns(uint256[] memory v){v=new uint256[](4);v[0]=a;v[1]=b;v[2]=c;v[3]=d;}
}
