// GoShare project main.go
package main

import (
	"flag"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/julienschmidt/httprouter"
)

var Port = flag.Int("port", 80, "Port to listen on")
var BaseDir = flag.String("basedir", ".", "Base directory to work from")

var AbsBaseDir string

func main() {
	listenAddr := &net.TCPAddr{
		Port: *Port,
	}
	l, err := net.ListenTCP("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	r := httprouter.New()

	r.GET("/file/*file", DownloadHandler)
	r.POST("/upload", UploadHandler)
	r.GET("/dir/*dir", DirHandler)
	r.Handler("GET", "/", http.RedirectHandler("/dir/", http.StatusFound))

	r.Handler("GET", "/assets/simplegrid.css", SimpleGrid)

	AbsBaseDir, err = filepath.Abs(*BaseDir)
	if err != nil {
		log.Fatal(err)
	}

	s := &http.Server{}
	s.Handler = r
	s.Serve(l)
}

var DirTemplate = template.Must(template.New("dir").Funcs(template.FuncMap{
	"Size": func(size int64) string {
		return humanize.Bytes(uint64(size))
	},
}).Parse(`<html>
<head>
<link rel="stylesheet" type="text/css" href="/assets/simplegrid.css"/>
<style>
.files {

}
.files .filename {
	width: 75%;
}
table, th, td {
	border: 1px solid black;
}
table {
	border-collapse: collapse;
}
</style>
</head>
<body>
<div class="grid">
<div class="col-3-12">
<form action="/upload" method="post" enctype="multipart/form-data">
    Select file to upload:
    <input type="file" name="file" id="fileToUpload"><br/>
    <input type="submit" value="Upload" name="submit">
	<input type="hidden" value="{{.Dir}}">
</form>

</div>
<table class="files col-9-12">
<tr>
<th class="filename">Filename</th><th class="size">Size</th>
</tr>
{{range .Items}}
<tr>
<td class="filename"><a href="/{{if .IsDir}}dir{{else}}file{{end}}/{{$.Dir}}/{{.Name}}">{{.Name}}</a></td><td class="size">{{Size .Size}}</td>
</tr>
{{end}}
</table>
</div>
</body>
</html>`))

type ItemsSort []os.FileInfo

func (is ItemsSort) Len() int {
	return len(is)
}

func (is ItemsSort) Less(i, j int) bool {
	if is[i].IsDir() != is[j].IsDir() {
		return is[i].IsDir()
	}
	return strings.ToLower(is[i].Name()) < strings.ToLower(is[j].Name())
}

func (is ItemsSort) Swap(i, j int) {
	is[i], is[j] = is[j], is[i]
}

func DirHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	dir := ps.ByName("dir")
	dir = filepath.Join(AbsBaseDir, dir)
	dir = filepath.Clean(dir)
	if !filepath.HasPrefix(dir, AbsBaseDir) {
		http.Error(w, "Error: Attempted access outside of base directory", http.StatusBadRequest)
		return
	}

	f, err := os.Open(dir)
	if err != nil {
		http.Error(w, "Error: Unable to open dir", http.StatusBadRequest)
		return
	}
	defer f.Close()
	items, err := f.Readdir(0)
	if err != nil {
		http.Error(w, "Error: Unable to read dir", http.StatusBadRequest)
		return
	}
	dir, _ = filepath.Rel(AbsBaseDir, dir)

	itemssort := ItemsSort(items)

	sort.Sort(itemssort)

	err = DirTemplate.Execute(w, struct {
		Items []os.FileInfo
		Dir   template.HTMLAttr
	}{
		Items: itemssort,
		Dir:   template.HTMLAttr(dir),
	})
	if err != nil {
		log.Println(err)
	}
}

func DownloadHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	file := ps.ByName("file")
	file = filepath.Join(AbsBaseDir, file)
	file = filepath.Clean(file)
	if !filepath.HasPrefix(file, AbsBaseDir) {
		http.Error(w, "Error: Attempted access outside of base directory", http.StatusBadRequest)
		return
	}

	f, err := os.Open(file)
	if err != nil {
		http.Error(w, "Error: Unable to open file", http.StatusBadRequest)
		return
	}
	defer f.Close()

	w.Header().Add("Content-Disposition", "attachment; filename=\""+filepath.Base(file)+"\"")
	w.Header().Add("Content-Type", "application/octet-stream")
	io.Copy(w, f)
}

func UploadHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	dir := r.FormValue("dir")

	dir = filepath.Join(AbsBaseDir, dir)
	dir = filepath.Clean(dir)
	if !filepath.HasPrefix(dir, AbsBaseDir) {
		http.Error(w, "Error: Attempted access outside of base directory", http.StatusBadRequest)
		return
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error occured while trying to upload file", http.StatusBadRequest)
	}
	defer f.Close()

	file := filepath.Base(fh.Filename)

	file = filepath.Join(dir, file)

	of, err := os.Create(file)
	if err != nil {
		http.Error(w, "Error: Unable to open file", http.StatusBadRequest)
		return
	}
	defer of.Close()

	io.Copy(of, f)
	dir, _ = filepath.Rel(AbsBaseDir, dir)
	http.Redirect(w, r, "/dir/"+dir, http.StatusFound)
}
