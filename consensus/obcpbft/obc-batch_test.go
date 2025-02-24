/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package obcpbft

import (
	"testing"
	"time"

	"github.com/hyperledger/fabric/consensus"

	"github.com/spf13/viper"
)

func (op *obcBatch) getPBFTCore() *pbftCore {
	return op.pbft
}

func obcBatchHelper(id uint64, config *viper.Viper, stack consensus.Stack) pbftConsumer {
	// It's not entirely obvious why the compiler likes the parent function, but not newObcBatch directly
	return newObcBatch(id, config, stack)
}

func TestNetworkBatch(t *testing.T) {
	batchSize := 2
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcBatchHelper, func(ce *consumerEndpoint) {
		ce.consumer.(*obcBatch).batchSize = batchSize
	})
	defer net.stop()

	broadcaster := net.endpoints[generateBroadcaster(validatorCount)].getHandle()
	err := net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), broadcaster)
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}

	net.process()

	if l := len(net.endpoints[0].(*consumerEndpoint).consumer.(*obcBatch).batchStore); l != 1 {
		t.Fatalf("%d message expected in primary's batchStore, found %d", 1, l)
	}

	err = net.endpoints[2].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(2), broadcaster)
	net.process()

	if l := len(net.endpoints[0].(*consumerEndpoint).consumer.(*obcBatch).batchStore); l != 0 {
		t.Fatalf("%d messages expected in primary's batchStore, found %d", 0, l)
	}

	for _, ep := range net.endpoints {
		ce := ep.(*consumerEndpoint)
		block, err := ce.consumer.(*obcBatch).stack.GetBlock(1)
		if nil != err {
			t.Fatalf("Replica %d executed requests, expected a new block on the chain, but could not retrieve it : %s", ce.id, err)
		}
		numTrans := len(block.Transactions)
		if numTrans != batchSize {
			t.Fatalf("Replica %d executed %d requests, expected %d",
				ce.id, numTrans, batchSize)
		}
		if numTxResults := len(block.NonHashData.TransactionResults); numTxResults != 1 /*numTrans*/ {
			t.Fatalf("Replica %d has %d txResults, expected %d", ce.id, numTxResults, numTrans)
		}
	}
}

func TestBatchCustody(t *testing.T) {
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, func(id uint64, config *viper.Viper, stack consensus.Stack) pbftConsumer {
		config.Set("general.batchsize", "2")
		config.Set("general.timeout.batch", "500ms")
		config.Set("general.timeout.request", "800ms")
		config.Set("general.timeout.viewchange", "1600ms")
		return newObcBatch(id, config, stack)
	})
	net.filterFn = func(src int, dst int, payload []byte) []byte {
		logger.Info("msg from %d to %d", src, dst)
		if src == 0 {
			return nil
		}
		return payload
	}

	go net.processContinually()
	r2 := net.endpoints[2].(*consumerEndpoint).consumer
	r2.RecvMsg(createOcMsgWithChainTx(1), net.endpoints[1].getHandle())
	r2.RecvMsg(createOcMsgWithChainTx(2), net.endpoints[1].getHandle())
	time.Sleep(6 * time.Second)
	net.stop()

	for i, inst := range net.endpoints {
		// Don't care about byzantine node 0
		if i == 0 {
			continue
		}
		inst := inst.(*consumerEndpoint)
		_, err := inst.consumer.(*obcBatch).stack.GetBlock(1)
		if err != nil {
			t.Errorf("Expected replica %d to have one block", inst.id)
			continue
		}
	}
}
