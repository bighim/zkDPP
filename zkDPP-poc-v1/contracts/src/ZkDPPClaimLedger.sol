// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;
import {IFieldHasher} from "./IFieldHasher.sol";
import {IZkVerifier} from "./IZkVerifier.sol";

// M8 main path; M7 remains an unchanged baseline.
contract ZkDPPClaimLedger {
 uint256 public constant TREE_DEPTH=32;
 uint256 public constant EPOCH_SIZE=600;
 uint8 public constant NOTE=1;
 uint8 public constant VOUCHER=2;
 uint8 public constant DPP=3;
 uint8 public constant ACTIVE=0;
 uint8 public constant FROZEN=1;
 uint8 public constant REVOKED=2;
 uint8 public constant PROCESS=6;
 uint8 public constant ISSUE=8;
 uint256 private constant FIELD=52435875175126190479447740508185965837690552500527637822603658699938581184513;
 uint256 public constant POLICY_REF_TAG=51780253709110118800953810799850621917828020824397005216617999075326973004822;
     struct Tree {
        mapping(uint256 => bool) acceptedRoots;
        mapping(uint256 => uint256) nodes;
        uint256[TREE_DEPTH + 1] zeroes;
        uint256 leafCount;
        uint256 currentRoot;
    }

    struct PolicyAuthority {
        address account;
        bool enabled;
    }

    struct PolicyFamily {
        uint8 eventKind;
        bool exists;
    }

    struct PolicyReservation {
        uint64 authorityId;
        uint64 policyId;
        uint64 version;
        uint8 eventKind;
        bool exists;
    }

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


 struct ObjectRef { uint8 objectType; uint256 rawId; }
 struct AuditCipher { uint256 r1X; uint256 r1Y; uint256[] encryptedParents; uint256[] encryptedOutputNfs; }
 struct AuditRecord { uint8 eventKind; uint256 policyRef; ObjectRef[] outputRefs; uint256 r1X; uint256 r1Y; uint256[] encryptedParents; uint256[] encryptedOutputNfs; }
 address public immutable admin;
 address public immutable statusAuthority;
 IFieldHasher public immutable fieldHasher;
 IZkVerifier public immutable auditEntryVerifier;
IZkVerifier public immutable auditTransferVerifier;
IZkVerifier public immutable auditProceedVerifier;
IZkVerifier public immutable auditRecallVerifier;
IZkVerifier public immutable auditMergeVerifier;
IZkVerifier public immutable auditSplitVerifier;
IZkVerifier public immutable auditProcessVerifier;
IZkVerifier public immutable m8ExitVerifier;
 mapping(address=>bool) public entryIssuers;
 mapping(uint256=>bool) public commitments;
 mapping(uint256=>bool) public voucherCommitments;
 mapping(uint8=>mapping(uint256=>uint256)) public producerOf;
 mapping(uint256=>uint256) public noteSpentIn;
 mapping(uint256=>uint256) public voucherSpentIn;
 mapping(uint256=>uint8) public noteStatusByNf;
 mapping(uint256=>uint8) public voucherStatusByNf;
 uint256 public nextAuditRecordId=1;
 mapping(uint256=>AuditRecord) private auditRecords;
     uint64 public nextAuthorityId = 1;
    mapping(address => uint64) public authorityIdOf;
    mapping(uint64 => PolicyAuthority) public policyAuthorities;
    mapping(uint64 => uint64) public nextPolicyId;
    mapping(uint64 => mapping(uint64 => uint64)) public nextPolicyVersion;
    mapping(uint64 => mapping(uint64 => PolicyFamily)) public policyFamilies;
    mapping(uint256 => PolicyReservation) public policyReservations;
    mapping(uint256 => PolicyRecord) public policyRecords;
    mapping(uint256 => mapping(uint256 => bool)) public policyGrants;
    mapping(uint256 => mapping(uint256 => uint256)) public claimRecordOf;
    mapping(uint256 => mapping(uint256 => uint8)) public claimStatus;

 Tree private noteTree;
 Tree private voucherTree;
 event EntryIssuerUpdated(address indexed account,bool allowed);
 event NoteAppended(uint256 indexed cm,uint256 index,uint256 root);
 event VoucherAppended(uint256 indexed rv,uint256 index,uint256 root);
 event NoteExited(uint256 indexed nf);
 event AuditRecorded(uint256 indexed auditRecordId);
 event DPPFinalized(uint256 indexed dppCommitment,uint256 indexed auditRecordId);
 event ClaimIssued(uint256 indexed dppCommitment,uint256 indexed issuePolicyRef,uint256 indexed auditRecordId);
 event ClaimStatusChanged(uint256 indexed dppCommitment,uint256 indexed issuePolicyRef,uint8 oldStatus,uint8 newStatus);
 event NullifierStatusChanged(uint8 indexed objectType,uint256 indexed spendValue,uint8 oldStatus,uint8 newStatus);
     event PolicyAuthorityRegistered(uint64 indexed authorityId, address indexed account);
    event PolicyReserved(
        uint256 indexed policyRef,
        uint64 indexed authorityId,
        uint64 policyId,
        uint64 version,
        uint8 eventKind
    );
    event PolicyRegistered(uint256 indexed policyRef, address indexed verifierRef, bytes32 vkHash);
    event PolicyGrantUpdated(
        uint256 indexed policyRef, uint256 indexed policyScopeRef, bool allowed
    );
    event PolicyDisabled(uint256 indexed policyRef);


 error NotAdmin(); error NotEntryIssuer(); error InvalidAccount(); error DuplicateCommitment(); error DuplicateVoucher();
 error InvalidRoot(); error DuplicateInput(); error SpentNullifier(); error ResolvedVoucher(); error InvalidEpoch();
 error InvalidProof(); error TreeFull(); error UnknownPolicy(); error DisabledPolicy(); error InvalidPolicyRecord();
 error MissingPolicyGrant(); error NotPolicyAuthority(); error DuplicateAuthority(); error InvalidReservation();
 error WrongAuthority(); error DuplicatePolicy(); error InvalidVersion(); error NotStatusAuthority(); error InactiveObject();
 error InvalidStatus(); error InvalidObjectType(); error InvalidField(); error InvalidAuditShape(); error UnknownRecord();
 error InvalidEventKind(); error DuplicateDPP(); error DuplicateClaim(); error UnknownDPP(); error UnknownClaim();
 error InvalidClaimSource(); error InvalidClaimPolicy(); error InvalidIssueGrant(); error UnknownPolicyFamily();  error PreviousVersionNotRegistered();  error UnknownPolicyReservation();  error InvalidLeafIndex();
 constructor(address[8] memory verifierAddresses,address hasher,address authority){
  if(hasher.code.length==0||authority==address(0)) revert InvalidAccount();
  for(uint i;i<8;i++){if(verifierAddresses[i].code.length==0)revert InvalidAccount();}
  admin=msg.sender;statusAuthority=authority;fieldHasher=IFieldHasher(hasher);
  auditEntryVerifier=IZkVerifier(verifierAddresses[0]);
auditTransferVerifier=IZkVerifier(verifierAddresses[1]);
auditProceedVerifier=IZkVerifier(verifierAddresses[2]);
auditRecallVerifier=IZkVerifier(verifierAddresses[3]);
auditMergeVerifier=IZkVerifier(verifierAddresses[4]);
auditSplitVerifier=IZkVerifier(verifierAddresses[5]);
auditProcessVerifier=IZkVerifier(verifierAddresses[6]);
m8ExitVerifier=IZkVerifier(verifierAddresses[7]);
  _initTree(noteTree);_initTree(voucherTree);
 }
 function setEntryIssuer(address account,bool allowed) external {
  if(msg.sender!=admin)revert NotAdmin();if(account==address(0))revert InvalidAccount();
  entryIssuers[account]=allowed;emit EntryIssuerUpdated(account,allowed);
 }
 function noteNullifiers(uint256 nf) external view returns(bool){return noteSpentIn[nf]!=0;}
 function voucherNullifiers(uint256 nf) external view returns(bool){return voucherSpentIn[nf]!=0;}
 function getAuditRecord(uint256 id) external view returns(AuditRecord memory){
  if(id==0||id>=nextAuditRecordId)revert UnknownRecord();return auditRecords[id];
 }
 function _active(uint8 t,uint256 s) private view {
  if(t==NOTE){if(noteSpentIn[s]!=0)revert SpentNullifier();if(noteStatusByNf[s]!=ACTIVE)revert InactiveObject();}
  else {if(voucherSpentIn[s]!=0)revert ResolvedVoucher();if(voucherStatusByNf[s]!=ACTIVE)revert InactiveObject();}
 }
 function _fresh(ObjectRef[] memory refs) private view{
  for(uint i;i<refs.length;i++){
   if(refs[i].objectType==NOTE){if(commitments[refs[i].rawId])revert DuplicateCommitment();}
   else {if(voucherCommitments[refs[i].rawId])revert DuplicateVoucher();}
   for(uint j;j<i;j++){if(refs[i].objectType==refs[j].objectType&&refs[i].rawId==refs[j].rawId)revert DuplicateCommitment();}
  }
 }
 function _refs(uint8 t1,uint256 id1) private pure returns(ObjectRef[] memory r){r=new ObjectRef[](1);r[0]=ObjectRef(t1,id1);}
 function _refs2(uint8 t1,uint256 id1,uint8 t2,uint256 id2) private pure returns(ObjectRef[] memory r){r=new ObjectRef[](2);r[0]=ObjectRef(t1,id1);r[1]=ObjectRef(t2,id2);}
 function _eventKind() private pure returns(uint8){
  if(msg.sig==ZkDPPClaimLedger.entry.selector)return 0;
  if(msg.sig==ZkDPPClaimLedger.transfer.selector)return 1;
  if(msg.sig==ZkDPPClaimLedger.proceed.selector)return 2;
  if(msg.sig==ZkDPPClaimLedger.recall.selector)return 3;
  if(msg.sig==ZkDPPClaimLedger.merge.selector)return 4;
  if(msg.sig==ZkDPPClaimLedger.split.selector)return 5;
  if(msg.sig==ZkDPPClaimLedger.process.selector)return 6;
  if(msg.sig==ZkDPPClaimLedger.exit.selector)return 7;
  revert InvalidAuditShape();
 }
 function _selected(uint8 kind) private view returns(IZkVerifier){
  if(kind==0)return auditEntryVerifier;
if(kind==1)return auditTransferVerifier;
if(kind==2)return auditProceedVerifier;
if(kind==3)return auditRecallVerifier;
if(kind==4)return auditMergeVerifier;
if(kind==5)return auditSplitVerifier;
if(kind==6)return auditProcessVerifier;
if(kind==7)return m8ExitVerifier;
  revert InvalidAuditShape();
 }
 function _complete(bytes memory proof,uint256[] memory p,uint256[] memory spends,ObjectRef[] memory refs,AuditCipher memory a) private returns(uint256 aid){
  uint8 kind=_eventKind();
  uint parents=kind==0?0:kind==4?2:kind==6?3:1;
  uint outputs=kind==7?0:kind==1||kind==5||kind==6?2:1;
  if(a.encryptedParents.length!=parents||a.encryptedOutputNfs.length!=outputs||refs.length!=outputs)revert InvalidAuditShape();
  _fresh(refs);
  uint256[] memory inputs=new uint256[](p.length+2+parents+outputs);
  for(uint i;i<p.length;i++)inputs[i]=p[i];
  inputs[p.length]=a.r1X;inputs[p.length+1]=a.r1Y;
  for(uint i;i<parents;i++)inputs[p.length+2+i]=a.encryptedParents[i];
  for(uint i;i<outputs;i++)inputs[p.length+2+parents+i]=a.encryptedOutputNfs[i];
  for(uint i;i<inputs.length;i++)if(inputs[i]>=FIELD)revert InvalidField();
  _verify(_selected(kind),proof,inputs);
  aid=nextAuditRecordId++;
  bool voucherInput=kind==2||kind==3;
  for(uint i;i<spends.length;i++){if(voucherInput)voucherSpentIn[spends[i]]=aid;else noteSpentIn[spends[i]]=aid;}
  AuditRecord storage rec=auditRecords[aid];
  rec.eventKind=kind;rec.policyRef=kind==6?p[0]:0;rec.r1X=a.r1X;rec.r1Y=a.r1Y;
  rec.encryptedParents=a.encryptedParents;rec.encryptedOutputNfs=a.encryptedOutputNfs;
  for(uint i;i<refs.length;i++){
   ObjectRef memory ref=refs[i];rec.outputRefs.push(ref);
   if(ref.objectType==NOTE){commitments[ref.rawId]=true;_append(noteTree,ref.rawId,false);}
   else {voucherCommitments[ref.rawId]=true;_append(voucherTree,ref.rawId,true);}
   producerOf[ref.objectType][ref.rawId]=aid;
  }
  emit AuditRecorded(aid);
 }
 function entry(bytes calldata proof,uint256 cm,AuditCipher calldata a) external returns(uint256){
  if(!entryIssuers[msg.sender])revert NotEntryIssuer();
  return _complete(proof,_one(cm),new uint256[](0),_refs(NOTE,cm),a);
 }
 function exit(bytes calldata proof,uint256 root,uint256 nf,uint256 dppCommitment,AuditCipher calldata a) external returns(uint256 aid){
  if(!noteTree.acceptedRoots[root])revert InvalidRoot();_active(NOTE,nf);
  if(dppCommitment>=FIELD)revert InvalidField();
  if(producerOf[DPP][dppCommitment]!=0)revert DuplicateDPP();
  if(a.encryptedParents.length!=1||a.encryptedOutputNfs.length!=0)revert InvalidAuditShape();
  uint256[] memory inputs=new uint256[](6);
  inputs[0]=root;inputs[1]=nf;inputs[2]=dppCommitment;inputs[3]=a.r1X;inputs[4]=a.r1Y;inputs[5]=a.encryptedParents[0];
  for(uint i;i<inputs.length;i++)if(inputs[i]>=FIELD)revert InvalidField();
  _verify(m8ExitVerifier,proof,inputs);
  aid=nextAuditRecordId++;
  noteSpentIn[nf]=aid;
  AuditRecord storage rec=auditRecords[aid];
  rec.eventKind=7;rec.outputRefs.push(ObjectRef(DPP,dppCommitment));rec.r1X=a.r1X;rec.r1Y=a.r1Y;rec.encryptedParents=a.encryptedParents;
  producerOf[DPP][dppCommitment]=aid;
  emit AuditRecorded(aid);emit NoteExited(nf);emit DPPFinalized(dppCommitment,aid);
 }
 function issue(bytes calldata proof,uint256 issuePolicyRef,uint256 dppCommitment) external returns(uint256 aid){
  if(issuePolicyRef>=FIELD||dppCommitment>=FIELD)revert InvalidField();
  uint256 exitRecordId=producerOf[DPP][dppCommitment];
  if(exitRecordId==0)revert UnknownDPP();
  if(auditRecords[exitRecordId].eventKind!=7)revert InvalidClaimSource();
  PolicyRecord memory policy=policyRecords[issuePolicyRef];
  if(policy.authorityId==0)revert UnknownPolicy();if(!policy.enabled)revert DisabledPolicy();
  if(policy.eventKind!=ISSUE||policy.inputArity!=1||policy.outputArity!=1)revert InvalidClaimPolicy();
  if(claimRecordOf[dppCommitment][issuePolicyRef]!=0)revert DuplicateClaim();
  uint256[] memory inputs=new uint256[](2);inputs[0]=issuePolicyRef;inputs[1]=dppCommitment;
  _verify(IZkVerifier(policy.verifierRef),proof,inputs);
  aid=nextAuditRecordId++;
  AuditRecord storage rec=auditRecords[aid];rec.eventKind=ISSUE;rec.policyRef=issuePolicyRef;
  claimRecordOf[dppCommitment][issuePolicyRef]=aid;
  emit AuditRecorded(aid);emit ClaimIssued(dppCommitment,issuePolicyRef,aid);
 }
 function verifyClaim(uint256 dppCommitment,uint256 issuePolicyRef) external view returns(bool registered,uint8 status,uint256 issueAuditRecordId){
  issueAuditRecordId=claimRecordOf[dppCommitment][issuePolicyRef];
  registered=issueAuditRecordId!=0;status=claimStatus[dppCommitment][issuePolicyRef];
 }
 function setClaimStatus(uint256 dppCommitment,uint256 issuePolicyRef,uint8 newStatus) external{
  if(msg.sender!=statusAuthority)revert NotStatusAuthority();
  if(claimRecordOf[dppCommitment][issuePolicyRef]==0)revert UnknownClaim();
  uint8 oldStatus=claimStatus[dppCommitment][issuePolicyRef];
  if(!((oldStatus==ACTIVE&&newStatus==FROZEN)||(oldStatus==FROZEN&&(newStatus==ACTIVE||newStatus==REVOKED))))revert InvalidStatus();
  claimStatus[dppCommitment][issuePolicyRef]=newStatus;
  emit ClaimStatusChanged(dppCommitment,issuePolicyRef,oldStatus,newStatus);
 }
 function transfer(bytes calldata proof,uint256 root,uint256 nf,uint256 rvNew,uint256 cmChange,uint256 transferEpoch,uint256 deltaEpoch,AuditCipher calldata a) external returns(uint256){
  if(!noteTree.acceptedRoots[root])revert InvalidRoot();_active(NOTE,nf);
  if(transferEpoch!=currentEpoch())revert InvalidEpoch();
  uint256[] memory p=new uint256[](6);p[0]=root;p[1]=nf;p[2]=rvNew;p[3]=cmChange;p[4]=transferEpoch;p[5]=deltaEpoch;
  return _complete(proof,p,_one(nf),_refs2(VOUCHER,rvNew,NOTE,cmChange),a);
 }
 function proceed(bytes calldata proof,uint256 root,uint256 nf,uint256 cm,AuditCipher calldata a) external returns(uint256){
  if(!voucherTree.acceptedRoots[root])revert InvalidRoot();_active(VOUCHER,nf);
  return _complete(proof,_three(root,nf,cm),_one(nf),_refs(NOTE,cm),a);
 }
 function recall(bytes calldata proof,uint256 root,uint256 nf,uint256 cm,uint256 epoch,AuditCipher calldata a) external returns(uint256){
  if(!voucherTree.acceptedRoots[root])revert InvalidRoot();_active(VOUCHER,nf);if(epoch!=currentEpoch())revert InvalidEpoch();
  return _complete(proof,_four(root,nf,cm,epoch),_one(nf),_refs(NOTE,cm),a);
 }
 function merge(bytes calldata proof,uint256 root,uint256 nf1,uint256 nf2,uint256 cm,AuditCipher calldata a) external returns(uint256){
  if(!noteTree.acceptedRoots[root])revert InvalidRoot();if(nf1==nf2)revert DuplicateInput();_active(NOTE,nf1);_active(NOTE,nf2);
  uint256[] memory spends=new uint256[](2);spends[0]=nf1;spends[1]=nf2;
  return _complete(proof,_four(root,nf1,nf2,cm),spends,_refs(NOTE,cm),a);
 }
 function split(bytes calldata proof,uint256 root,uint256 nf,uint256 cm1,uint256 cm2,AuditCipher calldata a) external returns(uint256){
  if(!noteTree.acceptedRoots[root])revert InvalidRoot();_active(NOTE,nf);
  return _complete(proof,_four(root,nf,cm1,cm2),_one(nf),_refs2(NOTE,cm1,NOTE,cm2),a);
 }
 function process(bytes calldata proof,uint256 policyRef,uint256 scope,uint256 root,uint256[3] calldata nfs,uint256[2] calldata cms,AuditCipher calldata a) external returns(uint256){
  PolicyRecord memory rec=policyRecords[policyRef];
  if(rec.authorityId==0)revert UnknownPolicy();if(!rec.enabled)revert DisabledPolicy();
  if(rec.eventKind!=PROCESS||rec.inputArity!=3||rec.outputArity!=2||rec.verifierRef!=address(auditProcessVerifier))revert InvalidPolicyRecord();
  if(!policyGrants[policyRef][scope])revert MissingPolicyGrant();
  if(!noteTree.acceptedRoots[root])revert InvalidRoot();
  uint256[] memory p=new uint256[](8);p[0]=policyRef;p[1]=scope;p[2]=root;
  uint256[] memory spends=new uint256[](3);
  for(uint i;i<3;i++){_active(NOTE,nfs[i]);p[i+3]=nfs[i];spends[i]=nfs[i];for(uint j;j<i;j++)if(nfs[i]==nfs[j])revert DuplicateInput();}
  p[6]=cms[0];p[7]=cms[1];
  return _complete(proof,p,spends,_refs2(NOTE,cms[0],NOTE,cms[1]),a);
 }
 function setStatus(uint8 objectType,uint256 spendValue,uint8 newStatus) external{
  if(msg.sender!=statusAuthority)revert NotStatusAuthority();
  if(objectType!=NOTE&&objectType!=VOUCHER)revert InvalidObjectType();
  if(spendValue>=FIELD)revert InvalidField();
  if((objectType==NOTE?noteSpentIn[spendValue]:voucherSpentIn[spendValue])!=0)revert SpentNullifier();
  uint8 oldStatus=objectType==NOTE?noteStatusByNf[spendValue]:voucherStatusByNf[spendValue];
  if(!((oldStatus==ACTIVE&&newStatus==FROZEN)||(oldStatus==FROZEN&&(newStatus==ACTIVE||newStatus==REVOKED))))revert InvalidStatus();
  if(objectType==NOTE)noteStatusByNf[spendValue]=newStatus;else voucherStatusByNf[spendValue]=newStatus;
  emit NullifierStatusChanged(objectType,spendValue,oldStatus,newStatus);
 }
     function registerPolicyAuthority(address account) external returns (uint64 authorityId) {
        if (msg.sender != admin) revert NotAdmin();
        if (account == address(0)) revert InvalidAccount();
        if (authorityIdOf[account] != 0) revert DuplicateAuthority();
        authorityId = nextAuthorityId++;
        authorityIdOf[account] = authorityId;
        policyAuthorities[authorityId] = PolicyAuthority(account, true);
        nextPolicyId[authorityId] = 1;
        emit PolicyAuthorityRegistered(authorityId, account);
    }

    function reservePolicy(uint8 eventKind) external returns (uint256 policyRef) {
        uint64 authorityId = _authority(msg.sender);
        if (eventKind != PROCESS && eventKind != ISSUE) revert InvalidEventKind();
        uint64 policyId = nextPolicyId[authorityId]++;
        uint64 version = 1;
        policyFamilies[authorityId][policyId] = PolicyFamily(eventKind, true);
        nextPolicyVersion[authorityId][policyId] = 2;
        policyRef = _computePolicyRef(eventKind, authorityId, policyId, version);
        policyReservations[policyRef] =
            PolicyReservation(authorityId, policyId, version, eventKind, true);
        emit PolicyReserved(policyRef, authorityId, policyId, version, eventKind);
    }

    function reservePolicyVersion(uint64 policyId) external returns (uint256 policyRef) {
        uint64 authorityId = _authority(msg.sender);
        PolicyFamily memory family = policyFamilies[authorityId][policyId];
        if (!family.exists) revert UnknownPolicyFamily();
        uint64 version = nextPolicyVersion[authorityId][policyId];
        uint256 previousRef = _computePolicyRef(
            family.eventKind, authorityId, policyId, version - 1
        );
        if (policyRecords[previousRef].authorityId == 0) revert PreviousVersionNotRegistered();
        nextPolicyVersion[authorityId][policyId] = version + 1;
        policyRef = _computePolicyRef(family.eventKind, authorityId, policyId, version);
        policyReservations[policyRef] =
            PolicyReservation(authorityId, policyId, version, family.eventKind, true);
        emit PolicyReserved(policyRef, authorityId, policyId, version, family.eventKind);
    }

    function registerPolicy(
        uint256 policyRef,
        uint8 inputArity,
        uint8 outputArity,
        bytes32 vkHash,
        address verifierRef
    ) external {
        uint64 authorityId = _authority(msg.sender);
        PolicyReservation memory reservation = policyReservations[policyRef];
        if (!reservation.exists || reservation.authorityId != authorityId) {
            revert UnknownPolicyReservation();
        }
        if (policyRecords[policyRef].authorityId != 0) revert DuplicatePolicy();
        if (
            inputArity == 0 || outputArity == 0 || vkHash == bytes32(0)
                || verifierRef.code.length == 0
        ) {
            revert InvalidPolicyRecord();
        }
        if (
            reservation.eventKind == PROCESS
                && (inputArity != 3
                    || outputArity != 2
                    || verifierRef != address(auditProcessVerifier))
        ) revert InvalidPolicyRecord();
        if (reservation.eventKind == ISSUE && (inputArity != 1 || outputArity != 1)) {
            revert InvalidPolicyRecord();
        }
        policyRecords[policyRef] = PolicyRecord(
            authorityId,
            reservation.policyId,
            reservation.version,
            reservation.eventKind,
            inputArity,
            outputArity,
            vkHash,
            verifierRef,
            true
        );
        emit PolicyRegistered(policyRef, verifierRef, vkHash);
    }

    function setPolicyGrant(uint256 policyRef, uint256 policyScopeRef, bool allowed) external {
        PolicyRecord memory record = policyRecords[policyRef];
        if (record.authorityId == 0) revert UnknownPolicy();
        if (_authority(msg.sender) != record.authorityId) revert NotPolicyAuthority();
        if (record.eventKind != PROCESS) revert InvalidIssueGrant();
        if (policyScopeRef == 0) revert InvalidPolicyRecord();
        policyGrants[policyRef][policyScopeRef] = allowed;
        emit PolicyGrantUpdated(policyRef, policyScopeRef, allowed);
    }

    function disablePolicy(uint256 policyRef) external {
        PolicyRecord storage record = policyRecords[policyRef];
        if (record.authorityId == 0) revert UnknownPolicy();
        if (_authority(msg.sender) != record.authorityId) revert NotPolicyAuthority();
        if (!record.enabled) revert DisabledPolicy();
        record.enabled = false;
        emit PolicyDisabled(policyRef);
    }


     function currentEpoch() public view returns (uint256) {
        return block.timestamp / EPOCH_SIZE;
    }

    function currentNoteRoot() external view returns (uint256) {
        return noteTree.currentRoot;
    }

    function currentVoucherRoot() external view returns (uint256) {
        return voucherTree.currentRoot;
    }

    function acceptedNoteRoot(uint256 root) external view returns (bool) {
        return noteTree.acceptedRoots[root];
    }

    function acceptedVoucherRoot(uint256 root) external view returns (bool) {
        return voucherTree.acceptedRoots[root];
    }

    function noteLeafCount() external view returns (uint256) {
        return noteTree.leafCount;
    }

    function voucherLeafCount() external view returns (uint256) {
        return voucherTree.leafCount;
    }

    function noteLeaf(uint256 index) external view returns (uint256) {
        if (index >= noteTree.leafCount) revert InvalidLeafIndex();
        return noteTree.nodes[_nodeIndex(0, index)];
    }

    function voucherLeaf(uint256 index) external view returns (uint256) {
        if (index >= voucherTree.leafCount) revert InvalidLeafIndex();
        return voucherTree.nodes[_nodeIndex(0, index)];
    }

    function getNotePath(uint256 index) external view returns (uint256, uint256[] memory) {
        return _getPath(noteTree, index);
    }

    function getVoucherPath(uint256 index) external view returns (uint256, uint256[] memory) {
        return _getPath(voucherTree, index);
    }

    function computePolicyRef(uint8 eventKind, uint64 authorityId, uint64 policyId, uint64 version)
        external
        view
        returns (uint256)
    {
        return _computePolicyRef(eventKind, authorityId, policyId, version);
    }


     function _authority(address account) private view returns (uint64 authorityId) {
        authorityId = authorityIdOf[account];
        if (authorityId == 0 || !policyAuthorities[authorityId].enabled) revert NotPolicyAuthority();
    }

    function _computePolicyRef(uint8 eventKind, uint64 authorityId, uint64 policyId, uint64 version)
        private
        view
        returns (uint256 state)
    {
        state = fieldHasher.compress(0, POLICY_REF_TAG);
        state = fieldHasher.compress(state, eventKind);
        state = fieldHasher.compress(state, authorityId);
        state = fieldHasher.compress(state, policyId);
        state = fieldHasher.compress(state, version);
    }

    function _initTree(Tree storage tree) private {
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            tree.zeroes[level + 1] = fieldHasher.compress(tree.zeroes[level], tree.zeroes[level]);
        }
        tree.currentRoot = tree.zeroes[TREE_DEPTH];
        tree.acceptedRoots[tree.currentRoot] = true;
    }

    function _append(Tree storage tree, uint256 leaf, bool isVoucher) private {
        if (tree.leafCount >= (uint256(1) << TREE_DEPTH)) revert TreeFull();
        uint256 index = tree.leafCount;
        uint256 position = index;
        uint256 current = leaf;
        tree.nodes[_nodeIndex(0, position)] = current;
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            uint256 left;
            uint256 right;
            if (position & 1 == 0) {
                left = current;
                right = _node(tree, level, position + 1);
            } else {
                left = _node(tree, level, position - 1);
                right = current;
            }
            current = fieldHasher.compress(left, right);
            position >>= 1;
            tree.nodes[_nodeIndex(level + 1, position)] = current;
        }
        tree.leafCount = index + 1;
        tree.currentRoot = current;
        tree.acceptedRoots[current] = true;
        if (isVoucher) emit VoucherAppended(leaf, index, current);
        else emit NoteAppended(leaf, index, current);
    }

    function _getPath(Tree storage tree, uint256 index)
        private
        view
        returns (uint256 root, uint256[] memory siblings)
    {
        if (index >= tree.leafCount) revert InvalidLeafIndex();
        siblings = new uint256[](TREE_DEPTH);
        uint256 position = index;
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            siblings[level] = _node(tree, level, position ^ 1);
            position >>= 1;
        }
        return (tree.currentRoot, siblings);
    }

    function _node(Tree storage tree, uint256 level, uint256 position)
        private
        view
        returns (uint256)
    {
        uint256 value = tree.nodes[_nodeIndex(level, position)];
        return value == 0 ? tree.zeroes[level] : value;
    }

    function _nodeIndex(uint256 level, uint256 position) private pure returns (uint256) {
        return (uint256(1) << (TREE_DEPTH - level)) - 1 + position;
    }

    function _verify(IZkVerifier verifier, bytes memory proof, uint256[] memory inputs)
        private
        view
    {
        try verifier.Verify(proof, inputs) returns (bool valid) {
            if (!valid) revert InvalidProof();
        }
            catch {
            revert InvalidProof();
        }
    }

    function _one(uint256 a) private pure returns (uint256[] memory v) {
        v = new uint256[](1);
        v[0] = a;
    }

    function _three(uint256 a, uint256 b, uint256 c) private pure returns (uint256[] memory v) {
        v = new uint256[](3);
        v[0] = a;
        v[1] = b;
        v[2] = c;
    }

    function _four(uint256 a, uint256 b, uint256 c, uint256 d)
        private
        pure
        returns (uint256[] memory v)
    {
        v = new uint256[](4);
        v[0] = a;
        v[1] = b;
        v[2] = c;
        v[3] = d;
    }
}
