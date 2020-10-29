package myhttp

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/patrickhener/goshs/internal/myca"
	"github.com/patrickhener/goshs/internal/myclipboard"
	"github.com/patrickhener/goshs/internal/myconfig"
	"github.com/patrickhener/goshs/internal/mylog"
	"github.com/patrickhener/goshs/internal/myutils"
	"github.com/patrickhener/goshs/internal/mywebsock"

	"github.com/phogolabs/parcello"

	// This will import for bundling with parcello
	_ "github.com/patrickhener/goshs/static"
)

type indexTemplate struct {
	Clipboard    *myclipboard.Clipboard
	GoshsVersion string
	Directory    *directory
}

type directory struct {
	RelPath        string
	AbsPath        string
	IsSubdirectory bool
	Back           string
	Content        []item
}

type item struct {
	URI                 string
	Name                string
	IsDir               bool
	IsSymlink           bool
	SymlinkTarget       string
	Ext                 string
	DisplaySize         string
	SortSize            int64
	DisplayLastModified string
	SortLastModified    time.Time
}

// WebServer holds the WebServer information
type WebServer struct {
	IP           string
	Port         int
	Webroot      string
	SharedConfig *myconfig.SharedConfig
	Version      string
	Hub          *mywebsock.Hub
	Clipboard    *myclipboard.Clipboard
	HTTPServer   *http.Server
}

type httperror struct {
	ErrorCode    int
	ErrorMessage string
	AbsPath      string
	GoshsVersion string
}

// BasicAuthMiddleware is a middleware to handle the basic auth
func (ws *WebServer) BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)

		username, password, authOK := r.BasicAuth()
		if authOK == false {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		if username != "gopher" || password != ws.SharedConfig.Pass {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})

}

// Start will start the file server
func (ws *WebServer) Start() error {
	// Setup routing with gorilla/mux
	mux := mux.NewRouter()
	mux.PathPrefix("/425bda8487e36deccb30dd24be590b8744e3a28a8bb5a57d9b3fcd24ae09ad3c/").HandlerFunc(ws.static)
	// Websocket
	mux.PathPrefix("/14644be038ea0118a1aadfacca2a7d1517d7b209c4b9674ee893b1944d1c2d54/ws").HandlerFunc(ws.socket)
	// Clipboard
	mux.PathPrefix("/14644be038ea0118a1aadfacca2a7d1517d7b209c4b9674ee893b1944d1c2d54/download").HandlerFunc(ws.cbDown)
	mux.PathPrefix("/cf985bddf28fed5d5c53b069d6a6ebe601088ca6e20ec5a5a8438f8e1ffd9390/").HandlerFunc(ws.bulkDownload)
	mux.Methods(http.MethodPost).HandlerFunc(ws.upload)
	mux.PathPrefix("/").HandlerFunc(ws.handler)

	// construct server
	add := fmt.Sprintf("%+v:%+v", ws.IP, ws.Port)
	ws.HTTPServer = &http.Server{
		Addr:    add,
		Handler: mux,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// init clipboard
	ws.Clipboard = myclipboard.New()

	// init websocket hub
	ws.Hub = mywebsock.NewHub(ws.Clipboard)
	go ws.Hub.Run()

	// Check BasicAuth and use middleware
	if ws.SharedConfig.Pass != "" {
		if !ws.SharedConfig.TLS {
			log.Printf("WARNING!: You are using basic auth without TLS. Your credentials will be transferred in cleartext. Consider using -t, too.\n")
		}
		log.Printf("Using 'gopher:%+v' as basic auth\n", ws.SharedConfig.Pass)
		// Use middleware
		mux.Use(ws.BasicAuthMiddleware)
	}

	// Check if TLS
	if ws.SharedConfig.TLS {
		// Check if selfsigned
		if ws.SharedConfig.SelfSigned {
			var (
				err            error
				fingerprint256 string
				fingerprint1   string
			)
			ws.SharedConfig.TLSConfig, fingerprint256, fingerprint1, err = myca.Setup()
			if err != nil {
				log.Printf("Unable to start TLS enabled server: %+v\n", err)
				return err
			}
			ws.HTTPServer.TLSConfig = ws.SharedConfig.TLSConfig
			log.Printf("Serving HTTPS on %+v port %+v from %+v with TLS enabled and self-signed certificate\n", ws.IP, ws.Port, ws.Webroot)
			log.Println("WARNING! Be sure to check the fingerprint of certificate")
			log.Printf("SHA-256 Fingerprint: %+v\n", fingerprint256)
			log.Printf("SHA-1   Fingerprint: %+v\n", fingerprint1)
			log.Panic(ws.HTTPServer.ListenAndServeTLS("", ""))
			return nil
		}
		if ws.SharedConfig.Cert == "" || ws.SharedConfig.Key == "" {
			log.Fatalln("You need to provide server.key and server.crt if -t and not -ss")
		}

		fingerprint256, fingerprint1, err := myca.ParseAndSum(ws.SharedConfig.Cert)
		if err != nil {
			log.Printf("Unable to start TLS enabled server: %+v\n", err)
			return err
		}

		log.Printf("Serving HTTPS on %+v port %+v from %+v with TLS enabled server key: %+v, server cert: %+v\n", ws.IP, ws.Port, ws.Webroot, ws.SharedConfig.Key, ws.SharedConfig.Cert)
		log.Println("INFO! You provided a certificate and might want to check the fingerprint nonetheless")
		log.Printf("SHA-256 Fingerprint: %+v\n", fingerprint256)
		log.Printf("SHA-1   Fingerprint: %+v\n", fingerprint1)

		log.Panic(ws.HTTPServer.ListenAndServeTLS(ws.SharedConfig.Cert, ws.SharedConfig.Key))
		return nil
	}
	log.Printf("Serving HTTP on %+v port %+v from %+v\n", ws.IP, ws.Port, ws.Webroot)
	log.Panic(ws.HTTPServer.ListenAndServe())
	return nil
}

// Stop will gracefully shutdown the web server
func (ws *WebServer) Stop(ctx context.Context) {
	ws.HTTPServer.Shutdown(ctx)
}

// socket will handle the socket connection
func (ws *WebServer) socket(w http.ResponseWriter, req *http.Request) {
	mywebsock.ServeWS(ws.Hub, w, req)
}

// clipboardAdd will handle the add request for adding text to the clipboard
func (ws *WebServer) cbDown(w http.ResponseWriter, req *http.Request) {
	filename := fmt.Sprintf("%+v-clipboard.json", int32(time.Now().Unix()))
	contentDisposition := fmt.Sprintf("attachment; filename=\"%s\"", filename)
	// Handle as download
	w.Header().Add("Content-Type", "application/octet-stream")
	w.Header().Add("Content-Disposition", contentDisposition)
	content, err := ws.Clipboard.Download()
	if err != nil {
		ws.handleError(w, req, err, 500)
	}

	if _, err := w.Write(content); err != nil {
		log.Printf("ERROR: Error writing response to browser: %+v", err)
	}
}

// static will give static content for style and function
func (ws *WebServer) static(w http.ResponseWriter, req *http.Request) {
	// Check which file to serve
	upath := req.URL.Path
	staticPath := strings.SplitAfterN(upath, "/", 3)[2]
	// Load file with parcello
	staticFile, err := parcello.Open(staticPath)
	if err != nil {
		log.Printf("ERROR: static file: %+v cannot be loaded: %+v", staticPath, err)
	}

	// Read file
	staticContent, err := ioutil.ReadAll(staticFile)
	if err != nil {
		log.Printf("ERROR: static file: %+v cannot be read: %+v", staticPath, err)
	}

	// Get mimetype from extension
	contentType := myutils.MimeByExtension(staticPath)

	// Set mimetype and deliver to browser
	w.Header().Add("Content-Type", contentType)
	if _, err := w.Write(staticContent); err != nil {
		log.Printf("ERROR: Error writing response to browser: %+v", err)
	}
}

// handler is the function which actually handles dir or file retrieval
func (ws *WebServer) handler(w http.ResponseWriter, req *http.Request) {
	// Get url so you can extract Headline and title
	upath := req.URL.Path

	// Ignore default browser call to /favicon.ico
	if upath == "/favicon.ico" {
		return
	}

	// Define absolute path
	open := ws.Webroot + path.Clean(upath)

	// Check if you are in a dir
	// disable G304 (CWE-22): Potential file inclusion via variable
	// as we want a file inclusion here
	// #nosec G304
	file, err := os.Open(open)
	if os.IsNotExist(err) {
		ws.handleError(w, req, err, http.StatusNotFound)
		return
	}
	if os.IsPermission(err) {
		ws.handleError(w, req, err, http.StatusInternalServerError)
		return
	}
	if err != nil {
		// Handle general error
		log.Println(err)
		return
	}
	// disable G307 (CWE-703): Deferring unsafe method "Close" on type "*os.File"
	// #nosec G307
	defer file.Close()

	// Log request
	mylog.LogRequest(req.RemoteAddr, req.Method, req.URL.Path, req.Proto, http.StatusOK)

	// Switch and check if dir
	stat, _ := file.Stat()
	if stat.IsDir() {
		ws.processDir(w, req, file, upath)
	} else {
		ws.sendFile(w, req, file)
	}
}

// upload handles the POST request to upload files
func (ws *WebServer) upload(w http.ResponseWriter, req *http.Request) {
	// Get url so you can extract Headline and title
	upath := req.URL.Path

	// construct target path
	targetpath := strings.Split(upath, "/")
	targetpath = targetpath[:len(targetpath)-1]
	target := strings.Join(targetpath, "/")

	// Parse request
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		log.Printf("Error parsing multipart request: %+v", err)
		return
	}

	// Get ref to the parsed multipart form
	m := req.MultipartForm

	// Get the File Headers
	files := m.File["files"]

	for i := range files {
		file, err := files[i].Open()
		defer file.Close()
		if err != nil {
			log.Printf("Error retrieving the file: %+v\n", err)
		}

		filename := files[i].Filename

		// Sanitize filename (No path traversal)
		filenameSlice := strings.Split(filename, "/")
		filenameClean := filenameSlice[len(filenameSlice)-1]

		// Construct absolute savepath
		savepath := fmt.Sprintf("%s%s/%s", ws.Webroot, target, filenameClean)

		// Create file to write to
		if _, err := os.Create(savepath); err != nil {
			log.Println("ERROR: Not able to create file on disk")
			ws.handleError(w, req, err, http.StatusInternalServerError)
		}

		// Read file from post body
		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			log.Println("ERROR: Not able to read file from request")
			ws.handleError(w, req, err, http.StatusInternalServerError)
		}

		// Write file to disk
		if err := ioutil.WriteFile(savepath, fileBytes, os.ModePerm); err != nil {
			log.Println("ERROR: Not able to write file to disk")
			ws.handleError(w, req, err, http.StatusInternalServerError)
		}

	}

	// Log request
	mylog.LogRequest(req.RemoteAddr, req.Method, req.URL.Path, req.Proto, http.StatusOK)

	// Redirect back from where we came from
	http.Redirect(w, req, target, http.StatusSeeOther)
}

// bulkDownload will provide zip archived download bundle of multiple selected files
func (ws *WebServer) bulkDownload(w http.ResponseWriter, req *http.Request) {
	// make slice and query files from request
	var filesCleaned []string
	files := req.URL.Query()["file"]

	// Handle if no files are selected
	if len(files) <= 0 {
		ws.handleError(w, req, errors.New("You need to select a file before you can download a zip archive"), 404)
	}

	// Clean file paths and fill slice
	// Also sanitize path (No path traversal)
	// If .. in single string just skip file
	for _, file := range files {
		fileCleaned, _ := url.QueryUnescape(file)
		if strings.Contains(fileCleaned, "..") {
			// Just skip this file
			continue
		}
		filesCleaned = append(filesCleaned, fileCleaned)
	}

	// Construct filename to download
	filename := fmt.Sprintf("%+v_goshs_download.zip", int32(time.Now().Unix()))

	// Set header and serve file
	contentDispo := fmt.Sprintf("attachment; filename=\"%s\"", filename)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDispo)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Expires", "0")

	// Define Zip writer
	resultZip := zip.NewWriter(w)
	defer resultZip.Close()

	// Path walker for recursion
	walker := func(filepath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// disable G304 (CWE-22): Potential file inclusion via variable
		// as we want a file inclusion here
		// #nosec G304
		file, err := os.Open(filepath)
		if err != nil {
			return err
		}
		// disable G307 (CWE-703): Deferring unsafe method "Close" on type "*os.File"
		// #nosec G307
		defer file.Close()

		// filepath is ws.Webroot + file relative path
		// this would result in a lot of nested folders
		// so we are stripping ws.Webroot again from the structure of the zip file
		// Leaving us with the relative path of the file
		zippath := strings.ReplaceAll(filepath, ws.Webroot, "")
		f, err := resultZip.Create(zippath[1:])
		if err != nil {
			return err
		}

		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}

		return nil
	}

	// Loop over files and add to zip
	for _, file := range filesCleaned {
		err := filepath.Walk(path.Join(ws.Webroot, file), walker)
		if err != nil {
			log.Printf("Error creating zip file: %+v", err)
		}
	}

	// Close Zip Writer and Flush to http.ResponseWriter
	if err := resultZip.Close(); err != nil {
		log.Println(err)
	}
}

func (ws *WebServer) processDir(w http.ResponseWriter, req *http.Request, file *os.File, relpath string) {
	// Read directory FileInfo
	fis, err := file.Readdir(-1)
	if err != nil {
		ws.handleError(w, req, err, http.StatusNotFound)
		return
	}

	// Create empty slice
	items := make([]item, 0, len(fis))
	// Iterate over FileInfo of dir
	for _, fi := range fis {
		var item = item{}
		// Need to set this up here for directories to work
		item.Name = fi.Name()
		item.Ext = strings.ToLower(myutils.ReturnExt(fi.Name()))
		// Add / to name if dir
		if fi.IsDir() {
			// Check if special path exists as dir on disk and do not add
			if myutils.CheckSpecialPath(fi.Name()) {
				continue
			}
			item.Name += "/"
			item.IsDir = true
			item.Ext = ""
		}
		// Set item fields
		item.URI = url.PathEscape(path.Join(relpath, fi.Name()))
		item.DisplaySize = myutils.ByteCountDecimal(fi.Size())
		item.SortSize = fi.Size()
		item.DisplayLastModified = fi.ModTime().Format("Mon Jan _2 15:04:05 2006")
		item.SortLastModified = fi.ModTime()
		// Check and resolve symlink
		if fi.Mode()&os.ModeSymlink != 0 {
			item.IsSymlink = true
			item.SymlinkTarget, err = os.Readlink(path.Join(ws.Webroot, relpath, fi.Name()))
			if err != nil {
				log.Printf("Error resolving symlink: %+v", err)
			}
		}
		// Add to items slice
		items = append(items, item)
	}

	// Sort slice all lowercase
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	// Template parsing and writing to browser
	indexFile, err := parcello.Open("templates/index.html")
	if err != nil {
		log.Printf("Error opening embedded file: %+v", err)
	}
	fileContent, err := ioutil.ReadAll(indexFile)
	if err != nil {
		log.Printf("Error opening embedded file: %+v", err)
	}

	// Construct directory for template
	d := &directory{
		RelPath: relpath,
		AbsPath: path.Join(ws.Webroot, relpath),
		Content: items,
	}
	if relpath != "/" {
		d.IsSubdirectory = true
		pathSlice := strings.Split(relpath, "/")
		if len(pathSlice) > 2 {
			pathSlice = pathSlice[1 : len(pathSlice)-1]

			var backString string = ""
			for _, part := range pathSlice {
				backString += "/" + part
			}
			d.Back = backString

		} else {
			d.Back = "/"
		}
	} else {
		d.IsSubdirectory = false
	}

	// Construct template
	tem := &indexTemplate{
		Directory:    d,
		GoshsVersion: ws.SharedConfig.GoshsVersion,
		Clipboard:    ws.Clipboard,
	}

	t := template.New("index")
	if _, err := t.Parse(string(fileContent)); err != nil {
		log.Printf("ERROR: Error parsing template: %+v", err)
	}
	if err := t.Execute(w, tem); err != nil {
		log.Printf("ERROR: Error executing template: %+v", err)
	}
}

func (ws *WebServer) sendFile(w http.ResponseWriter, req *http.Request, file *os.File) {
	// Extract download parameter
	download := req.URL.Query()
	if _, ok := download["download"]; ok {
		stat, err := file.Stat()
		if err != nil {
			log.Printf("Error reading file stats for download: %+v", err)
		}
		contentDisposition := fmt.Sprintf("attachment; filename=\"%s\"", stat.Name())
		// Handle as download
		w.Header().Add("Content-Type", "application/octet-stream")
		w.Header().Add("Content-Disposition", contentDisposition)
		if _, err := io.Copy(w, file); err != nil {
			log.Printf("ERROR: Error writing response to browser: %+v", err)
		}
	} else {
		// Write to browser
		if _, err := io.Copy(w, file); err != nil {
			log.Printf("ERROR: Error writing response to browser: %+v", err)
		}
	}
}

func (ws *WebServer) handleError(w http.ResponseWriter, req *http.Request, err error, status int) {
	// Set header to status
	w.WriteHeader(status)

	// Define empty error
	var e httperror

	// Log to console
	mylog.LogRequest(req.RemoteAddr, req.Method, req.URL.Path, req.Proto, status)

	// Construct error for template filling
	e.ErrorCode = status
	e.ErrorMessage = err.Error()
	e.AbsPath = path.Join(ws.Webroot, req.URL.Path)
	e.GoshsVersion = ws.SharedConfig.GoshsVersion

	// Template handling
	file, err := parcello.Open("templates/error.html")
	if err != nil {
		log.Printf("Error opening embedded file: %+v", err)
	}
	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("Error opening embedded file: %+v", err)
	}
	t := template.New("error")
	if _, err := t.Parse(string(fileContent)); err != nil {
		log.Printf("Error parsing the template: %+v", err)

	}
	if err := t.Execute(w, e); err != nil {
		log.Printf("Error executing the template: %+v", err)
	}
}
