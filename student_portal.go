package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/PAVAN2627/gofr/pkg/datasource/file/azureblob"
)

var az *azureblob.AzureBlob
var templates = template.Must(template.ParseFiles("upload.html", "filelist.html"))

func main() {
	var err error
	az, err = azureblob.NewAzureBlob(
    os.Getenv("AZURE_ACCOUNT"),
    os.Getenv("AZURE_KEY"),
    os.Getenv("AZURE_CONTAINER"),
)
	if err != nil {
		log.Fatal("Azure Blob init failed:", err)
	}

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/view", viewHandler)
	http.HandleFunc("/download", downloadHandler)
	http.HandleFunc("/delete", deleteHandler)

	fmt.Println("🚀 Server running at http://localhost:5055")
	log.Fatal(http.ListenAndServe(":5055", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "upload.html", nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)
	studentID := r.FormValue("student_id")
	folder := strings.Trim(r.FormValue("folder"), "/")
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, _ := io.ReadAll(file)
	path := fmt.Sprintf("students/%s/%s/%s", studentID, folder, handler.Filename)
	path = strings.ReplaceAll(path, "//", "/")

	err = az.Upload(context.Background(), path, data)
	if err != nil {
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "✅ File uploaded to %s", path)
}


func listHandler(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	prefix := fmt.Sprintf("students/%s/", studentID)

	allFiles, err := az.List(context.Background())
	if err != nil {
		http.Error(w, "List failed", http.StatusInternalServerError)
		return
	}

	type FileEntry struct {
		FileName    string
		ViewURL     string
		DownloadURL string
		DeleteURL   string
	}

	var files []FileEntry

	for _, fullPath := range allFiles {
		if strings.HasPrefix(fullPath, prefix) {
			relative := strings.TrimPrefix(fullPath, prefix)
			if relative == "" {
				continue
			}

			encoded := url.QueryEscape(relative)
			files = append(files, FileEntry{
				FileName:    relative,
				ViewURL:     fmt.Sprintf("/view?student_id=%s&file=%s", studentID, encoded),
				DownloadURL: fmt.Sprintf("/download?student_id=%s&file=%s", studentID, encoded),
				DeleteURL:   fmt.Sprintf("/delete?student_id=%s&file=%s", studentID, encoded),
			})
		}
	}

	err = templates.ExecuteTemplate(w, "filelist.html", struct {
		StudentID string
		Files     []FileEntry
	}{
		StudentID: studentID,
		Files:     files,
	})
	if err != nil {
		http.Error(w, "Template rendering error", http.StatusInternalServerError)
	}
}


func viewHandler(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	filePath := r.URL.Query().Get("file")
	path := fmt.Sprintf("students/%s/%s", studentID, filePath)

	data, err := az.Download(context.Background(), path)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Write(data)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	filePath := r.URL.Query().Get("file")
	path := fmt.Sprintf("students/%s/%s", studentID, filePath)

	data, err := az.Download(context.Background(), path)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filePath)
	w.Write(data)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	filePath := r.URL.Query().Get("file")
	path := fmt.Sprintf("students/%s/%s", studentID, filePath)

	err := az.Delete(context.Background(), path)
	if err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/list?student_id="+studentID, http.StatusSeeOther)
}
