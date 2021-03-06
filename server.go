/*
Copyright 2013 Tristan Wietsma

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

package jack

import (
	"github.com/tristanwietsma/metastore"
	"log"
	"net"
	"strconv"
)

// StartServer starts up a listened at 127.0.0.1:<port> with a key space distributed over a given number of buckets.
func StartServer(port uint, buckets uint) {

	if buckets == 0 {
		log.Fatalf("<buckets> be positive. got %d.\n", buckets)
	}

	var db metastore.MetaStore
	db.Init(buckets)
	log.Printf("created storage with %d buckets\n", buckets)

	// listening on localhost
	addr := "127.0.0.1:" + strconv.FormatUint(uint64(port), 10)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	log.Printf("server listening on port %d\n", port)

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConnection(c, &db)
	}
}

func closeConnection(c net.Conn) {
	log.Printf("[%s] closed connection\n", c.RemoteAddr())
	c.Close()
}

func handleConnection(c net.Conn, dbase *metastore.MetaStore) {
	defer closeConnection(c)

	fromAddr := c.RemoteAddr()
	log.Printf("[%s] new connection\n", fromAddr)

	buf := make([]byte, 1024)

	bucketIndex := dbase.GetHasher()

NEXTMESSAGE:

	_, err := c.Read(buf)
	if err != nil {
		return
	}

	msg, err := Parse(buf)
	if err != nil {
		return
	}

	i := bucketIndex(msg.key)

	switch msg.cmd {

	case GET:

		if value, ok := (*dbase).Bucket[i].Get(string(msg.key)); ok {
			b := []byte(value)
			b = append(b, EOM)
			_, err = c.Write(b)
		} else {
			_, err = c.Write([]byte{EOM})
		}
		if err != nil {
			return
		}
		goto NEXTMESSAGE

	case SET:

		(*dbase).Bucket[i].Set(string(msg.key), string(msg.arg))
		_, err = c.Write([]byte{SUCCESS})
		if err != nil {
			return
		}
		goto NEXTMESSAGE

	case DEL:

		(*dbase).Bucket[i].Delete(string(msg.key))
		_, err = c.Write([]byte{SUCCESS})
		if err != nil {
			return
		}
		goto NEXTMESSAGE

	case PUB:

		(*dbase).Bucket[i].Publish(string(msg.key), string(msg.arg))
		_, err = c.Write([]byte{SUCCESS})
		if err != nil {
			return
		}
		goto NEXTMESSAGE

	case SUB:

		outgoing := make(chan string)
		(*dbase).Bucket[i].Subscribe(string(msg.key), outgoing)
		for value := range outgoing {
			b := []byte(value)
			b = append(b, EOM)
			_, err := c.Write(b)
			if err != nil {
				close(outgoing)
				return
			}
		}

	default:
		return
	}
}
