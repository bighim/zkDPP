import json, statistics, hashlib, csv
from pathlib import Path
ROOT = Path(__file__).resolve().parent
def stats(xs):
    return {"n":len(xs),"median":statistics.median(xs),"min":min(xs),"max":max(xs),"mean":statistics.mean(xs)}
runs = [json.loads((ROOT/f"raw/native-run-{i:02d}.json").read_text()) for i in range(1,4)]
rows=[r for run in runs for r in run["rows"] if not r["Warmup"]]
assert len(rows)==120
arms={}
for arm in ["A-sequential","A-parallel-2","B","C"]:
    rr=[r for r in rows if r["Arm"]==arm]; assert len(rr)==30
    arms[arm]={"proveMS":stats([r["ProveMS"] for r in rr]),"verifyMS":stats([r["VerifyMS"] for r in rr]),"runs":{}}
    for i in range(1,4):
        rs=[r for r in rr if r["Run"]==i];assert len(rs)==10
        arms[arm]["runs"][i]={"proveMS":stats([r["ProveMS"] for r in rs]),"verifyMS":stats([r["VerifyMS"] for r in rs])}
    for k in ["NativeProofBytes","SolidityProofBytes","PublicInputBytes","CanonicalNativeBytes"]:
        ss={r[k] for r in rr};assert len(ss)==1;arms[arm][k]=ss.pop()
setup=json.loads((ROOT/"setup.json").read_text())
keyrows={r["Name"]:r for r in setup["rows"]}
metadata={}
for arm,names in {"A":["path","eq","add"],"B":["B"],"C":["C"]}.items():
    metadata[arm]={"uniqueProvingKeyBytes":sum(keyrows[n]["Bytes"]["proving.key"] for n in names),"uniqueCircuits":[keyrows[n] for n in names]}
metadata["A"]["perProofSlotConstraints"]=3*keyrows["path"]["Constraints"]+2*keyrows["eq"]["Constraints"]+keyrows["add"]["Constraints"]
metadata["A"]["perProofSlotPaddedDomainSum"]=3*keyrows["path"]["Domain"]+2*keyrows["eq"]["Domain"]+keyrows["add"]["Domain"]
assert metadata["A"]["perProofSlotConstraints"]==keyrows["B"]["Constraints"]
def pct(old,new):return 100*(old-new)/old
comparisons={
    "AB_sequential_prove_reduction_percent":pct(arms["A-sequential"]["proveMS"]["median"],arms["B"]["proveMS"]["median"]),
    "AB_parallel_prove_reduction_percent":pct(arms["A-parallel-2"]["proveMS"]["median"],arms["B"]["proveMS"]["median"]),
    "AB_verify_reduction_percent":pct(arms["A-sequential"]["verifyMS"]["median"],arms["B"]["verifyMS"]["median"]),
    "BC_prove_C_over_B":arms["C"]["proveMS"]["median"]/arms["B"]["proveMS"]["median"],
    "BC_verify_C_over_B":arms["C"]["verifyMS"]["median"]/arms["B"]["verifyMS"]["median"],
    "AB_solidity_proof_reduction_percent":pct(arms["A-sequential"]["SolidityProofBytes"],arms["B"]["SolidityProofBytes"]),
    "AB_native_canonical_reduction_percent":pct(arms["A-sequential"]["CanonicalNativeBytes"],arms["B"]["CanonicalNativeBytes"])
}
evm_path=ROOT/"raw/evm.json"
evm=None
if evm_path.exists():
    er=json.loads(evm_path.read_text())["rows"];assert len(er)==90
    evm={}
    for arm in ["A","B","C"]:
        rr=[r for r in er if r["arm"]==arm];assert len(rr)==30
        assert all(r["returnedTrue"] and r["floorNotBinding"] and r["receipt"]["status"]=="0x1" for r in rr)
        sizes={r["canonicalABIBytes"] for r in rr};assert len(sizes)==1
        evm[arm]={"verificationExecutionGas":stats([r["verificationExecutionGas"] for r in rr]),"canonicalABIBytes":sizes.pop(),"runs":{i:stats([r["verificationExecutionGas"] for r in rr if r["run"]==i]) for i in range(1,4)}}
    comparisons["AB_gas_reduction_percent"]=pct(evm["A"]["verificationExecutionGas"]["median"],evm["B"]["verificationExecutionGas"]["median"])
    comparisons["AB_ABI_reduction_percent"]=pct(evm["A"]["canonicalABIBytes"],evm["B"]["canonicalABIBytes"])
    comparisons["BC_gas_C_over_B"]=evm["C"]["verificationExecutionGas"]["median"]/evm["B"]["verificationExecutionGas"]["median"]
summary={"environment":runs[0]["environment"],"official_native_rows":len(rows),"native":arms,"circuitMetadata":metadata,"comparisons":comparisons,"evm":evm}
(ROOT/"summary.json").write_text(json.dumps(summary,indent=2)+"\n")
with (ROOT/"summary.csv").open("w") as f:
    w=csv.writer(f);w.writerow(["arm","prove_median_ms","prove_min_ms","prove_max_ms","verify_median_ms","native_proof_B","solidity_proof_B","canonical_native_B","canonical_ABI_B","unique_PK_B","verification_execution_gas"])
    for arm,v in arms.items():
        key=arm[0];gas=evm[key]["verificationExecutionGas"]["median"] if evm else "";abi=evm[key]["canonicalABIBytes"] if evm else ""
        w.writerow([arm,v["proveMS"]["median"],v["proveMS"]["min"],v["proveMS"]["max"],v["verifyMS"]["median"],v["NativeProofBytes"],v["SolidityProofBytes"],v["CanonicalNativeBytes"],abi,metadata[key]["uniqueProvingKeyBytes"],gas])
print(json.dumps({"comparisons":comparisons,"prove":{a:v["proveMS"] for a,v in arms.items()},"verify":{a:v["verifyMS"] for a,v in arms.items()},"evm":evm},indent=2))

