package main

// seed_members fills an empty admin_members table with sample members so
// the panel's Members page has something to show on a fresh dev database.
// It refuses to run against a table that already has rows (deleted ones
// included), so it can never mix samples into real data.
//
//   make seed-members ENV=dev

import (
	"context"
	"log"
	"time"

	conf "fairchild-admin-panel-be/config"
	"fairchild-admin-panel-be/pkg/database"
)

var BuildEnv string

type sample struct {
	membershipType, category, branch            string
	firstName, lastName, middleName             string
	dateOfBirth, gender, email, mobile, address string
	guardianName, guardianMobile                string
	status, joinedAt                            string
}

var samples = []sample{
	{"coop", "regular", "main", "Maria", "Santos", "Reyes", "1988-03-14", "female", "maria.santos@example.com", "09171234501", "Brgy. Pulo, Cabuyao, Laguna", "", "", "active", "2019-06-03"},
	{"coop", "regular", "main", "Jose", "Dela Cruz", "", "1979-11-02", "male", "jose.delacruz@example.com", "09181234502", "Brgy. Banay-Banay, Cabuyao, Laguna", "", "", "active", "2015-01-20"},
	{"coop", "regular", "knowles", "Ana", "Garcia", "Lopez", "1992-07-25", "female", "ana.garcia@example.com", "09191234503", "Brgy. Mamatid, Cabuyao, Laguna", "", "", "active", "2021-09-14"},
	{"coop", "regular", "tiic", "Ramon", "Bautista", "", "1985-01-30", "male", "", "09201234504", "Brgy. Sala, Cabuyao, Laguna", "", "", "inactive", "2017-04-08"},
	{"coop", "associate", "main", "Liza", "Mendoza", "Cruz", "1995-05-19", "female", "liza.mendoza@example.com", "09211234505", "Brgy. Sto. Tomas, Calamba, Laguna", "", "", "active", "2022-02-11"},
	{"coop", "associate", "knowles", "Mark", "Villanueva", "", "1990-12-08", "male", "mark.villanueva@example.com", "09221234506", "Brgy. Parian, Calamba, Laguna", "", "", "active", "2020-10-05"},
	{"coop", "associate", "tiic", "Grace", "Torres", "Ramos", "1998-08-21", "female", "", "09231234507", "Brgy. Dila, Santa Rosa, Laguna", "", "", "active", "2023-03-27"},
	{"laboratory", "kiddie", "main", "Miguel", "Santos", "Reyes", "2016-04-10", "male", "", "", "Brgy. Pulo, Cabuyao, Laguna", "Maria Santos", "09171234501", "active", "2022-06-15"},
	{"laboratory", "kiddie", "knowles", "Sofia", "Garcia", "Lopez", "2018-09-03", "female", "", "", "Brgy. Mamatid, Cabuyao, Laguna", "Ana Garcia", "09191234503", "active", "2023-08-01"},
	{"laboratory", "aflatoun", "main", "Paolo", "Dela Cruz", "", "2011-02-17", "male", "", "", "Brgy. Banay-Banay, Cabuyao, Laguna", "Jose Dela Cruz", "09181234502", "active", "2020-07-22"},
	{"laboratory", "aflatoun", "tiic", "Bea", "Bautista", "", "2012-10-29", "female", "", "", "Brgy. Sala, Cabuyao, Laguna", "Ramon Bautista", "09201234504", "inactive", "2021-01-12"},
}

func main() {
	mysqlConf, err := conf.GetMySQLConfig(BuildEnv)
	if err != nil {
		log.Fatalf("MySQL config: %v", err)
	}
	db, err := database.MySQLInitConnector(mysqlConf, false)
	if err != nil {
		log.Fatalf("connect MySQL: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var existing int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_members`).Scan(&existing); err != nil {
		log.Fatalf("count admin_members (run migrations first?): %v", err)
	}
	if existing > 0 {
		log.Fatalf("admin_members already has %d rows - not seeding", existing)
	}

	null := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	for _, s := range samples {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO admin_members
				(membership_type, category, branch, first_name, last_name, middle_name,
				 date_of_birth, gender, nationality, email, mobile_number, address,
				 guardian_full_name, guardian_mobile_number, status, joined_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'Filipino', ?, ?, ?, ?, ?, ?, ?)`,
			s.membershipType, s.category, s.branch, s.firstName, s.lastName, null(s.middleName),
			s.dateOfBirth, s.gender, null(s.email), null(s.mobile), s.address,
			null(s.guardianName), null(s.guardianMobile), s.status, s.joinedAt,
		); err != nil {
			log.Fatalf("insert %s %s: %v", s.firstName, s.lastName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}
	log.Printf("seeded %d sample members", len(samples))
}
