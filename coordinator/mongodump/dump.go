package mongodump

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mongodb/mongo-tools/common/log"
	"github.com/mongodb/mongo-tools/common/progress"
	"github.com/mongodb/mongo-tools/common/util"
	"github.com/mongodb/mongo-tools/mongodump"
)

func DumpMongo(ctx context.Context, result chan string, mongoURL, dbName, backupDir string, progressManager progress.Manager) {
	optStr := []string{}
	optStr = append(optStr, "--uri", mongoURL+"/?authSource=admin", "--forceTableScan", "--db", dbName, "--gzip", "--archive="+backupDir+"/dump_"+fmt.Sprintf("%v", time.Now().Unix())+".archive", "--quiet")
	fmt.Println(optStr)
	opts, err := mongodump.ParseOptions(optStr, "", "")
	if err != nil {
		log.Logvf(log.Always, "error parsing command line options: %s", err.Error())
		log.Logvf(log.Always, util.ShortUsage("mongodump"))
		os.Exit(util.ExitFailure)
	}

	dump := mongodump.MongoDump{
		ToolOptions:     opts.ToolOptions,
		OutputOptions:   opts.OutputOptions,
		InputOptions:    opts.InputOptions,
		ProgressManager: progressManager,
	}
	dumpResult := make(chan string, 1)

	go func() {
		// var du TestDump
		// du.Pg = progressManager
		defer close(dumpResult)
		if err := dump.Init(); err != nil {
			// if err := du.Init(); err != nil {
			dumpResult <- err.Error()
			return
		}
		if err := dump.Dump(); err != nil {
			// if err := du.Dump(); err != nil {
			dumpResult <- err.Error()
			return
		} else {
			dumpResult <- "success"
			return
		}
	}()

	for {
		select {
		case r := <-dumpResult:
			result <- r
			return
		case <-ctx.Done():
			dump.HandleInterrupt()
			result <- "cancelled"
			return
		}
	}
}

type TestDump struct {
	Pg progress.Manager
}

func (td *TestDump) Dump() error {
	pg := progress.NewCounter(100)
	td.Pg.Attach("mongodump", pg)
	pg.Set(5)
	time.Sleep(time.Second * 1)
	pg.Set(25)
	time.Sleep(time.Second * 1)
	pg.Set(30)
	time.Sleep(time.Second * 1)
	pg.Set(40)
	time.Sleep(time.Second * 1)
	pg.Set(80)
	time.Sleep(time.Second * 1)
	pg.Set(80)
	time.Sleep(4 * time.Second)
	return nil
}

func (TestDump) Init() error {
	return nil
}
