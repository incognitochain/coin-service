package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	lvdbErrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func main() {
	argChainData := flag.String("chaindata", "", "set chain data folder")
	argShardHeight := flag.Uint64("height", 1, "set shard state")
	argShardID := flag.Int("shardid", 0, "set shardID")
	flag.Parse()
	chainData := *argChainData

	handles := -1
	cache := 8
	userDBPath := filepath.Join(chainData, "userdb")
	lvdb, err := leveldb.OpenFile(userDBPath, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	})
	if _, corrupted := err.(*lvdbErrors.ErrCorrupted); corrupted {
		lvdb, err = leveldb.RecoverFile(userDBPath, nil)
		if err != nil {
			panic(err)
		}
	}
	userDB := lvdb
	statePrefix := fmt.Sprintf("coin-processed-%v", *argShardID)
	value, err := userDB.Get([]byte(statePrefix), nil)
	if err != nil {
		panic(err)
	}
	log.Printf("changing shard %v from %v to %v", *argShardID, string(value), *argShardHeight)
	err = userDB.Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", *argShardHeight)), nil)
	if err != nil {
		panic(err)
	}
}
