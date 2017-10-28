package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

const MimeTypeGoogleAudio = "application/vnd.google-apps.audio"
const MimeTypeGoogleDocument = "application/vnd.google-apps.document"
const MimeTypeGoogleDrawing = "application/vnd.google-apps.drawing"
const MimeTypeGoogleFile = "application/vnd.google-apps.file"
const MimeTypeGoogleFolder = "application/vnd.google-apps.folder"
const MimeTypeGoogleForm = "application/vnd.google-apps.form"
const MimeTypeGoogleFusiontable = "application/vnd.google-apps.fusiontable"
const MimeTypeGoogleMap = "application/vnd.google-apps.map"
const MimeTypeGooglePhoto = "application/vnd.google-apps.photo"
const MimeTypeGooglePresentation = "application/vnd.google-apps.presentation"
const MimeTypeGoogleScript = "application/vnd.google-apps.script"
const MimeTypeGoogleSites = "application/vnd.google-apps.sites"
const MimeTypeGoogleSpreadsheet = "application/vnd.google-apps.spreadsheet"
const MimeTypeGoogleUnknown = "application/vnd.google-apps.unknown"
const MimeTypeGoogleVideo = "application/vnd.google-apps.video"
const MimeTypeGoogleDriveSdk = "application/vnd.google-apps.drive-sdk"

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
	return PathInCache("google.json"), nil
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

func GetDriveClient() *drive.Service {
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

func DriveSanitizeName(original_name, mime_type string) string {
	name := strings.Replace(original_name, "/", "\u2215", -1)

	switch mime_type {
	case MimeTypeGoogleAudio:
		return name + ".gdaud"
	case MimeTypeGoogleDocument:
		return name + ".gddoc"
	case MimeTypeGoogleDrawing:
		return name + ".gddraw"
	case MimeTypeGoogleFile:
		return name + ".gdfile"
	case MimeTypeGoogleFolder:
		return name
	case MimeTypeGoogleForm:
		return name + ".gdform"
	case MimeTypeGoogleFusiontable:
		return name + ".gdtable"
	case MimeTypeGoogleMap:
		return name + ".gdmap"
	case MimeTypeGooglePhoto:
		return name + ".gdphoto"
	case MimeTypeGooglePresentation:
		return name + ".gdslides"
	case MimeTypeGoogleScript:
		return name + ".gdscript"
	case MimeTypeGoogleSites:
		return name + ".gdsite"
	case MimeTypeGoogleSpreadsheet:
		return name + ".gdsheet"
	case MimeTypeGoogleUnknown:
		return name + ".gd"
	case MimeTypeGoogleVideo:
		return name + ".gdvideo"
	case MimeTypeGoogleDriveSdk:
		return name + ".gdsdk"
	default:
		return name
	}
}

func DriveUnambiguousName(id, original_name, mime_type string) string {
	return DriveSanitizeName(original_name+" ("+id+")", mime_type)
}
