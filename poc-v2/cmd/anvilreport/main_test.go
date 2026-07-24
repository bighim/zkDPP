package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestByteCounts(t *testing.T) {
	total, zero, nonZero, err := byteCounts("0x000102ff")
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || zero != 1 || nonZero != 3 {
		t.Fatalf("unexpected counts: total=%d zero=%d nonzero=%d", total, zero, nonZero)
	}
}

func TestParseHexOrDecimal(t *testing.T) {
	for input, want := range map[string]uint64{"0x10": 16, "42": 42} {
		got, err := parseHexOrDecimal(input)
		if err != nil || got != want {
			t.Fatalf("parse %s: got %d, err %v", input, got, err)
		}
	}
}

func TestBuildReportRejectsFailedReceipt(t *testing.T) {
	raw := syntheticBroadcast()
	raw.Receipts[0].Status = "0x0"
	_, err := buildReport(raw)
	if err == nil {
		t.Fatal("expected failed receipt to be rejected")
	}
}

func TestBuildReportCanonicalShape(t *testing.T) {
	raw := syntheticBroadcast()
	report, err := buildReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Deployments) != 10 || len(report.Calls) != 25 {
		t.Fatalf("unexpected output lengths: %d deployments, %d calls", len(report.Deployments), len(report.Calls))
	}
	if report.LogValidation.MTAppendCount != 28 || report.LogValidation.RVMTAppendCount != 7 {
		t.Fatalf("unexpected log counts: %+v", report.LogValidation)
	}
}

func syntheticBroadcast() broadcast {
	result := broadcast{}
	zkdpp := "0x000000000000000000000000000000000000d0d0"
	mtTopic := eventTopic("MTAppend(uint256,uint256,uint256)")
	rvmtTopic := eventTopic("RVMTAppend(uint256,uint256,uint256)")
	expectedTransactions := len(deploymentNames) + len(expectedCalls)
	for index := 0; index < expectedTransactions; index++ {
		hash := fmt.Sprintf("0x%064x", index+1)
		typeName := "CALL"
		contractAddress := ""
		functionName := ""
		to := zkdpp
		if index < 10 {
			typeName = "CREATE"
			contractAddress = fmt.Sprintf("0x%040x", index+1)
			to = ""
			if index == 9 {
				contractAddress = zkdpp
			}
		} else {
			functionName = expectedCalls[index-10].Event + "(bytes,uint256)"
		}
		envelope, _ := json.Marshal(transactionEnvelope{To: to, Input: "0x0001"})
		result.Transactions = append(result.Transactions, broadcastTransaction{
			Hash: hash, TransactionType: typeName, ContractAddress: contractAddress,
			Function: functionName, Transaction: envelope,
		})
		itemReceipt := receipt{
			Status: "0x1", TransactionHash: hash, BlockNumber: fmt.Sprintf("0x%x", index+1),
			GasUsed: "0x186a0", ContractAddress: contractAddress,
		}
		if index == 10 {
			for count := 0; count < 28; count++ {
				itemReceipt.Logs = append(itemReceipt.Logs, log{Address: zkdpp, Topics: []string{mtTopic}})
			}
			for count := 0; count < 7; count++ {
				itemReceipt.Logs = append(itemReceipt.Logs, log{Address: zkdpp, Topics: []string{rvmtTopic}})
			}
		}
		result.Receipts = append(result.Receipts, itemReceipt)
	}
	return result
}
