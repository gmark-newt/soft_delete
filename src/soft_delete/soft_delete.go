package main

import (
	"soft_delete/driver/database"
	//"github.com/codegangsta/cli"
	"encoding/csv"
	"errors"
	"fmt"
	jp "github.com/dustin/go-jsonpointer"
	"io"
	"log"
	"os"
	"path/filepath"
	"soft_delete/models"
	"strings"
)

type QuitRecord struct {
	FirstName   string `json:"-"` //col 0
	LastName    string `json:"-"` //col 1
	Email       string `json:"-"` //col 2
	Company     string `json:"-"` //col 3
	SoftDeleted string `json:"-"` //col 4
}

func main() {

	log.Print("Load Data from CSV")

	err := softDeleteQuitList("quit.csv")
	if err != nil {
		panic(err)
	}

	log.Print("End Soft Delete Quitters")
	return
}

func softDeleteQuitList(filename string) (err error) {

	//Check if CSV file
	ext := filepath.Ext(filename)
	if ext != ".csv" {
		err := errors.New("Error: Input file is not .csv")
		return err
	}

	//Open File
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	r := csv.NewReader(file)

	for i := 0; ; i++ {
		var qRecord QuitRecord

		row, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		//Skip header row
		if i == 0 {
			continue
		}

		qRecord = QuitRecord{
			FirstName:   row[0],
			LastName:    row[1],
			Email:       row[2],
			Company:     row[3],
			SoftDeleted: row[4],
		}

		// Begin TXs
		app := database.App.Begin()
		if app.Error != nil {
			log.Fatalf("Error starting transaction(s).\n\tApp: %v\n", app.Error)
			err = app.Error
			return err
		}

		//Grab UUID from user_emails via email
		var userEmail models.UserEmail
		err = app.Where("email = ?", qRecord.Email).Find(&userEmail).Error
		if err != nil {
			app.Rollback()
			log.Print("No Email data for this participant: ", qRecord.FirstName, " ", qRecord.LastName, ", ", qRecord.Email, " ~ Err: ", err)
			continue
		}

		//Grab Intake Record to compare name, with UUID from user_emails
		var userRecord models.Record
		err = app.Where("user_id = ? and entity_id in (SELECT id from entities where name = 'Intake')", userEmail.UserId).Find(&userRecord).Error
		if err != nil {
			app.Rollback()
			log.Print("No Intake Record data for this participant: ", qRecord.FirstName, " ", qRecord.LastName, ", ", qRecord.Email, " - with UserId: ", userEmail.UserId, " ~ Err: ", err)
			continue
		}

		FName := fmt.Sprintf("%v", jp.Get(userRecord.Meta, "/first_name"))
		LName := fmt.Sprintf("%v", jp.Get(userRecord.Meta, "/last_name"))

		if strings.ToUpper(FName) != strings.ToUpper(qRecord.FirstName) || strings.ToUpper(LName) != strings.ToUpper(qRecord.LastName) {
			app.Rollback()
			log.Print("Intake Record Names did not match: ", qRecord.FirstName, " ", qRecord.LastName, ", and from file:  ", FName, " ", LName)
			continue
		}

		//Make sure Association is correct
		var employer models.User
		var userAssociation models.Association

		err = app.Where("display_name = ?", qRecord.Company).Find(&employer).Error
		if err != nil {
			app.Rollback()
			log.Print("No User data for this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		err = app.Where("type = 'participant:employer' and (users #>> '{participant}')::uuid = ? and (users #>> '{employer}')::uuid = ?", userEmail.UserId, employer.UserId).Find(&userAssociation).Error
		if err != nil {
			app.Rollback()
			log.Print("No Employer Association for this Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		//If we have reached here, we can soft delete all records based on userEmail.UserId
		err = app.Where("user_id = ?", userEmail.UserId).Delete(&models.User{}).Error
		if err != nil {
			app.Rollback()
			log.Print("Error Deleting user for Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		err = app.Where("user_id = ?", userEmail.UserId).Delete(&models.UserState{}).Error
		if err != nil {
			app.Rollback()
			log.Print("Error Deleting user_states for Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		err = app.Where("user_id = ?", userEmail.UserId).Delete(&models.UserSettings{}).Error
		if err != nil {
			app.Rollback()
			log.Print("Error Deleting user_settings for Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		err = app.Where("user_id = ?", userEmail.UserId).Delete(&models.UserEmail{}).Error
		if err != nil {
			app.Rollback()
			log.Print("Error Deleting user_emails for Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		err = app.Where("user_id = ?", userEmail.UserId).Delete(&models.UserLog{}).Error
		if err != nil {
			app.Rollback()
			log.Print("Error Deleting user_logs for Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		err = app.Where("user_id = ?", userEmail.UserId).Delete(&models.UserAddress{}).Error
		if err != nil {
			app.Rollback()
			log.Print("Error Deleting user_addresses for Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		err = app.Where("(users #>> '{participant}')::uuid = ?", userEmail.UserId).Delete(&models.Association{}).Error
		if err != nil {
			app.Rollback()
			log.Print("Error Deleting associations for Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		err = app.Where("user_id = ?", userEmail.UserId).Delete(&models.Record{}).Error
		if err != nil {
			app.Rollback()
			log.Print("Error Deleting records for Person:", qRecord.FirstName, " ", qRecord.LastName, " and this Company: ", qRecord.Company, " ~ Err: ", err)
			continue
		}

		//Has not yet touched Validic? I don't know what's going on with that?

		err = app.Commit().Error
		if err != nil {
			app.Rollback()
			continue
		}

		log.Print("Successfully Soft-Deleted: ", userEmail.UserId, " - ", qRecord.FirstName, " ", qRecord.LastName, ", ", qRecord.Email)

	}
	return err

}
