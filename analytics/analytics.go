package analytics

import (
	"context"

	"github.com/jackc/pgx/v4"
)

func StartAnalyticService() {
	if err := connectDB(); err != nil {
		panic(err)
	}
	go startTradeVolumeAnalytic()
}

var connDB *pgx.Conn

func connectDB() error {
	var err error
	ctx := context.Background()
	connDB, err = pgx.Connect(ctx, "postgresql://admin:quest@localhost:8812/qdb")
	if err != nil {
		return err
	}
	return nil
}

func getDBConn() *pgx.Conn {
	if connDB == nil {
		connectDB()
	} else {
		if connDB.IsClosed() {
			connectDB()
		}
	}
	return connDB
}
