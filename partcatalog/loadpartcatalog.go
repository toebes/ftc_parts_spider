package partcatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

// https://developers.google.com/sheets/api/quickstart/go
// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err == nil {
			saveToken(tokFile, tok)
		}
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
		fmt.Println("visit https://developers.google.com/sheets/api/quickstart/go")
		return nil, fmt.Errorf("unable to read authorization code. Caused by: %v", err)
	}
	// authCode = ""

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web. Caused by %v", err)
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
		return fmt.Errorf("unable to cache oauth token. Caused by %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
	return nil
}

// LoadPartCatalog -
// Get Part# and URL from gobilda ALL spreadsheet:
// https://docs.google.com/spreadsheets/d/15XT3v9O0VOmyxqXrgR8tWDyb_CRLQT5-xPfWPdbx4RM/edit
func LoadPartCatalog(spreadsheetIDPtr *string, excludeFilter func(*PartData) bool) (*PartCatalogData, error) {

	var referenceData = NewPartCatalogData()

	if spreadsheetIDPtr == nil {
		fmt.Println("No SpreadsheetID was give, so no spreadsheet laoded")
		return referenceData, nil
	}
	spreadsheetID := *spreadsheetIDPtr

	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file. Caused by: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config. Caused by: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.New(client)
	if err != nil {
		return nil, err
	}

	readRange := "All"
	response, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to find 'ALL' sheet in spreadsheet %s. Caused by: %v", spreadsheetID, err)
	}

	if len(response.Values) == 0 {
		return nil, errors.New("no data in Spreadsheet")
	}

	referenceData.Partdata = make([]*PartData, 0, len(response.Values))
	for ii, cols := range response.Values {
		if ii == 0 {
			getColumnIndexes(referenceData, cols)
			continue // header row
		}
		partdata := getPartData(referenceData, cols)
		referenceData.addPart(partdata, excludeFilter)
	}
	return referenceData, nil
}
func getPartData(referenceData *PartCatalogData, cols []interface{}) *PartData {
	partdata := new(PartData)
	for jj, col := range cols {
		switch {
		case jj == referenceData.OrderColumnIndex:
			value, err := strconv.ParseUint(col.(string), 0, 32)
			if err == nil {
				partdata.Order = uint(value)
			} else {
				partdata.Order = 1
			}
		case jj == referenceData.SectionColumnIndex:
			partdata.Section = col.(string)
		case jj == referenceData.NameColumnIndex:
			partdata.Name = col.(string)
		case jj == referenceData.SKUColumnIndex:
			partdata.SKU = col.(string)
		case jj == referenceData.URLColumnIndex:
			partdata.URL = col.(string)
		case jj == referenceData.ModelURLColumnIndex:
			partdata.ModelURL = col.(string)
		case jj == referenceData.OnshapeURLColumnIndex:
			partdata.OnshapeURL = col.(string)
		case jj >= referenceData.ExtraColumnIndex && jj <= referenceData.ExtraColumnIndex+6:
			partdata.Extra[jj-referenceData.ExtraColumnIndex] = col.(string)
		case jj == referenceData.StatusColumnIndex:
			partdata.Status = col.(string)
		case jj == referenceData.NotesColumnIndex:
			partdata.Notes = col.(string)
		default:
		}
		partdata.SpiderStatus = PartNotFoundBySpider
	}
	return partdata
}
func getColumnIndexes(referenceData *PartCatalogData, cols []interface{}) {
	referenceData.OrderColumnIndex = -1
	referenceData.SectionColumnIndex = -1
	referenceData.NameColumnIndex = -1
	referenceData.SKUColumnIndex = -1
	referenceData.URLColumnIndex = -1
	referenceData.ModelURLColumnIndex = -1
	referenceData.ExtraColumnIndex = -1
	referenceData.OnshapeURLColumnIndex = -1
	referenceData.StatusColumnIndex = -1
	referenceData.NotesColumnIndex = -1

	for jj, col := range cols {
		switch col.(string) {
		case "Order":
			referenceData.OrderColumnIndex = jj
		case "Section":
			referenceData.SectionColumnIndex = jj
		case "Name":
			referenceData.NameColumnIndex = jj
		case "Part #":
			referenceData.SKUColumnIndex = jj
		case "URL":
			referenceData.URLColumnIndex = jj
		case "Model URL":
			referenceData.ModelURLColumnIndex = jj
		case "Onshape URL":
			referenceData.OnshapeURLColumnIndex = jj
		case "Extra 1":
			referenceData.ExtraColumnIndex = jj
		case "Status":
			referenceData.StatusColumnIndex = jj
		case "Model Status":
			referenceData.StatusColumnIndex = jj
		case "Notes":
			referenceData.NotesColumnIndex = jj
		default:
		}
	}
}
