package analyticdb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

var dbpool *pgxpool.Pool
var connectionStr string

func ConnectDB(connStr string) error {
	ctx := context.Background()
	var err error
	// connStr = "postgres://postgres:lam123@0.0.0.0:5432/postgres"
	dbpool, err = pgxpool.Connect(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		return err
	}
	connectionStr = connStr
	return nil
}

func getDBConn() *pgxpool.Pool {
	i := 0
retry:
	i++
	if i > 5 {
		panic(errors.New("max retry connecting database reached"))
	}
	if dbpool == nil {
		if err := ConnectDB(connectionStr); err != nil {
			log.Println(err)
			time.Sleep(1 * time.Second)
			goto retry
		}
	} else {
		if err := dbpool.Ping(context.Background()); err != nil {
			if err1 := ConnectDB(connectionStr); err1 != nil {
				log.Println(err1)
				time.Sleep(1 * time.Second)
				goto retry
			}
		}
	}
	return dbpool
}

func DropTable(table string, force bool) error {
	db := getDBConn()
	queryStr := "Drop table " + table
	if force {
		queryStr += " cascade"
	}
	_, err := db.Exec(context.Background(), queryStr)
	return err
}
