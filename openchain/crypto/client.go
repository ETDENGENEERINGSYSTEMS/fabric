/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package crypto

import (
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"sync"
)

// Private Variables

var (
	// Map of initialized clients
	clients = make(map[string]Client)

	// Sync
	clientMutex sync.Mutex
)

// Public Methods

// Register registers a client to the PKI infrastructure
func RegisterClient(name string, pwd []byte, enrollID, enrollPWD string) error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	log.Info("Registering [%s] with id [%s]...", enrollID, name)

	if clients[name] != nil {
		log.Info("Registering [%s] with id [%s]...done. Already initialized.", enrollID, name)
		return nil
	}

	client := new(clientImpl)
	if err := client.register(name, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s] with id [%s]: %s", enrollID, name, err)

		return err
	}
	err := client.close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Error("Registering [%s] with id [%s], failed closing: %s", enrollID, name, err)
	}

	log.Info("Registering [%s] with id [%s]...done!", enrollID, name)

	return nil
}

// Init initializes a client named name with password pwd
func InitClient(name string, pwd []byte) (Client, error) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	log.Info("Initializing [%s]...", name)

	if clients[name] != nil {
		log.Info("Client already initiliazied [%s].", name)

		return clients[name], nil
	}

	client := new(clientImpl)
	if err := client.init(name, pwd); err != nil {
		log.Error("Failed initialization [%s]: %s", name, err)

		return nil, err
	}

	clients[name] = client
	log.Info("Initializing [%s]...done!", name)

	return client, nil
}

// Close releases all the resources allocated by clients
func CloseClient(client Client) error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	return closeClientInternal(client)
}

// CloseAll closes all the clients initialized so far
func CloseAllClients() (bool, []error) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	log.Info("Closing all clients...")

	errs := make([]error, len(clients))
	for _, value := range clients {
		err := closeClientInternal(value)

		errs = append(errs, err)
	}

	log.Info("Closing all clients...done!")

	return len(errs) != 0, errs
}

// Private Methods

func closeClientInternal(client Client) error {
	id := client.GetName()
	log.Info("Closing client [%s]...", id)
	if _, ok := clients[id]; !ok {
		return utils.ErrInvalidReference
	}
	defer delete(clients, id)

	err := clients[id].(*clientImpl).close()

	log.Info("Closing client [%s]...done! [%s]", id, err)

	return err
}