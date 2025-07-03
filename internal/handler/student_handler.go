package handler

import (
	
	"net/http"
	"html/template"
	"gofr.dev/internal/service"
)

type Handler struct {
	service service.StudentService
}

func New(s service.StudentService) Handler {
	return Handler{service: s}
}

func (h Handler) Upload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "❌ Failed to read file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	err = h.service.UploadFile(r.Context(), header.Filename, file)
	if err != nil {
		http.Error(w, "❌ Upload error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther) // ✅ Redirect to list
}



func (h Handler) List(w http.ResponseWriter, r *http.Request) {
    files, err := h.service.ListFiles(r.Context())
    if err != nil {
        http.Error(w, "❌ List error: "+err.Error(), http.StatusInternalServerError)
        return
    }

    tmpl := template.Must(template.ParseFiles("templates/upload.html"))
    err = tmpl.Execute(w, files)
    if err != nil {
        http.Error(w, "❌ Template rendering error: "+err.Error(), http.StatusInternalServerError)
        return
    }
}

func (h Handler) Download(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	data, err := h.service.DownloadFile(r.Context(), filename)
	if err != nil {
		http.Error(w, "Download error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Write(data)
}

func (h Handler) Delete(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	filename := r.FormValue("file")
	if filename == "" {
		http.Error(w, "File name required", http.StatusBadRequest)
		return
	}

	err = h.service.DeleteFile(r.Context(), filename)
	if err != nil {
		http.Error(w, "Delete error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (h Handler) Home(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/upload.html"))

	files, err := h.service.ListFiles(r.Context())
	if err != nil {
		http.Error(w, "❌ Could not fetch file list", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, files)
}