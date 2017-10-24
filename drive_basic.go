package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"log"
	"net/http"
	"os"
)

const DevSecret = "{\"installed\":{\"client_id\":\"247137966113-i7t9f4qmg579dc5kjkoe9o1fiavemu1h.apps.googleusercontent.com\",\"project_id\":\"elevated-codex-175014\",\"auth_uri\":\"https://accounts.google.com/o/oauth2/auth\",\"token_uri\":\"https://accounts.google.com/o/oauth2/token\",\"auth_provider_x509_cert_url\":\"https://www.googleapis.com/oauth2/v1/certs\",\"client_secret\":\"zsJmWViFbtFh7tyCgTNHxINw\",\"redirect_uris\":[\"urn:ietf:wg:oauth:2.0:oob\",\"http://localhost\"]}}"

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {
	return file_in_config("quickstart.json")
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

var DriveCtx context.Context
var DriveConfig *oauth2.Config
var DriveClient *drive.Service

func get_node_info(node_id string) (*drive.File, bool) {
	f, err := DriveClient.Files.Get(node_id).
		Do()
	if err != nil {
		log.Printf("Unable to retrieve files: %+v", err)
		return nil, false
	}

	// Save cache

	return f, true
}

func get_nodes_ids_with_parent(parent_id string) ([]string, bool) {
	ans := make([]string, 0)

	f, err := DriveClient.Files.List().
		Fields("nextPageToken, files(id)").
		Q("'" + parent_id + "' in parents").
		Do()

	for {
		if err != nil {
			log.Printf("Unable to retrieve files: %v", err)
			return ans, false
		}
		for _, i := range f.Files {
			ans = append(ans, i.Id)
		}
		if f.NextPageToken == "" {
			return ans, true
		}
		f, err = DriveClient.Files.List().
			Fields("nextPageToken, files(id)").
			PageToken(f.NextPageToken).
			Q("'" + parent_id + "' in parents").
			Do()
	}

	return ans, true
}

func get_drive_client() *drive.Service {
	var err error
	DriveCtx = context.Background()
	DriveConfig, err = google.ConfigFromJSON([]byte(DevSecret), drive.DriveScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(DriveCtx, DriveConfig)
	srv, err := drive.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve drive Client %v", err)
	}

	return srv
}

func drive_test() {
	var err error
	DriveCtx = context.Background()
	DriveConfig, err = google.ConfigFromJSON([]byte(DevSecret), drive.DriveScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(DriveCtx, DriveConfig)

	srv, err := drive.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve drive Client %v", err)
	}

	r, err := srv.Files.List().
		Fields("nextPageToken, files(id, name)").
		Q("'root' in parents").
		Do()
	if err != nil {
		log.Fatalf("Unable to retrieve files: %v", err)
	}

	fmt.Println("Files:")
	if len(r.Files) > 0 {
		for _, i := range r.Files {
			fmt.Printf("%s (%s)\n", i.Name, i.Id)
		}
	} else {
		fmt.Println("No files found.")
	}
}
