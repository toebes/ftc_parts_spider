package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("Unable to read authorization code. Caused by: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve token from web. Caused by %v", err)
	}
	return tok, nil
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Unable to cache oauth token. Caused by %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
	return nil
}

// LoadStatusSpreadsheet -
// Get Part# and URL from gobilda ALL spreadsheet:
// https://docs.google.com/spreadsheets/d/15XT3v9O0VOmyxqXrgR8tWDyb_CRLQT5-xPfWPdbx4RM/edit
func LoadStatusSpreadsheet(spreadsheetIDPtr *string) (*ReferenceData, error) {

	var referenceData = new(ReferenceData)
	referenceData.partNumber = make(map[string]*PartData)
	referenceData.url = make(map[string]*PartData)

	if spreadsheetIDPtr == nil {
		fmt.Println("No SpreadsheetID was give, so no spreadsheet laoded")
		return referenceData, nil
	}
	spreadsheetID := *spreadsheetIDPtr

	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("Unable to read client secret file. Caused by: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client secret file to config. Caused by: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.New(client)
	if err != nil {
		return nil, err
	}

	readRange := "All"
	response, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("Unable to find 'ALL' sheet in spreadsheet %s. Caused by: %v", spreadsheetID, err)
	}

	if len(response.Values) == 0 {
		return nil, errors.New("no data in Spreadsheet")
	}

	referenceData.partdata = make([]*PartData, len(response.Values))
	for ii, cols := range response.Values {
		if ii == 0 {
			getColumnIndexes(referenceData, cols)
			continue // header row
		}

		partdata := new(PartData)
		for jj, col := range cols {
			switch {
			case jj == referenceData.orderColumnIndex:
				value, err := strconv.ParseUint(col.(string), 0, 32)
				if err == nil {
					partdata.Order = uint(value)
				} else {
					partdata.Order = 1
				}
			case jj == referenceData.sectionColumnIndex:
				partdata.Section = col.(string)
			case jj == referenceData.nameColumnIndex:
				partdata.Name = col.(string)
			case jj == referenceData.skuColumnIndex:
				partdata.SKU = col.(string)
			case jj == referenceData.urlColumnIndex:
				partdata.URL = col.(string)
			case jj == referenceData.modelURLColumnIndex:
				partdata.ModelURL = col.(string)
			case jj == referenceData.onShapeURLColumnIndex:
				partdata.OnshapeURL = col.(string)
			case jj >= referenceData.extraColumnIndex && jj <= referenceData.extraColumnIndex+6:
				partdata.Extra[jj-referenceData.extraColumnIndex] = col.(string)
			case jj == referenceData.statusColumnIndex:
				partdata.Status = col.(string)
			case jj == referenceData.notesColumnIndex:
				partdata.Notes = col.(string)
			default:
			}
			partdata.SpiderStatus = "Not Found by Spider"
			referenceData.partdata[ii] = partdata
		}
		if excludeFromMatch(partdata) {
			continue
		}
		dup, ok := referenceData.partNumber[partdata.SKU]
		if ok {
			fmt.Printf("row %d: duplicate part number '%s' found (original row %d)\n", ii, partdata.SKU, dup.Order)
		} else {
			referenceData.partNumber[partdata.SKU] = partdata
		}

		referenceData.url[partdata.URL] = partdata

	}

	return referenceData, nil
}
func getColumnIndexes(referenceData *ReferenceData, cols []interface{}) {
	referenceData.orderColumnIndex = -1
	referenceData.sectionColumnIndex = -1
	referenceData.nameColumnIndex = -1
	referenceData.skuColumnIndex = -1
	referenceData.urlColumnIndex = -1
	referenceData.modelURLColumnIndex = -1
	referenceData.extraColumnIndex = -1
	referenceData.onShapeURLColumnIndex = -1
	referenceData.statusColumnIndex = -1
	referenceData.notesColumnIndex = -1

	for jj, col := range cols {
		switch col.(string) {
		case "Order":
			referenceData.orderColumnIndex = jj
		case "Section":
			referenceData.sectionColumnIndex = jj
		case "Name":
			referenceData.nameColumnIndex = jj
		case "Part #":
			referenceData.skuColumnIndex = jj
		case "URL":
			referenceData.urlColumnIndex = jj
		case "Model URL":
			referenceData.modelURLColumnIndex = jj
		case "Onshape URL":
			referenceData.onShapeURLColumnIndex = jj
		case "Extra 1":
			referenceData.extraColumnIndex = jj
		case "Status":
			referenceData.statusColumnIndex = jj
		case "Model Status":
			referenceData.statusColumnIndex = jj
		case "Notes":
			referenceData.notesColumnIndex = jj
		default:
		}
	}
}
