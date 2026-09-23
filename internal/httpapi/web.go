package httpapi

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func serveWeb(api http.Handler, directory string) http.Handler {
	if directory == "" {
		return api
	}
	root := http.Dir(directory)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := path.Clean("/" + r.URL.Path)
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") ||
			name == "/api" || strings.HasPrefix(name, "/api/") ||
			(r.Method != http.MethodGet && r.Method != http.MethodHead) {
			api.ServeHTTP(w, r)
			return
		}
		file, info, err := openWebFile(root, name)
		if os.IsNotExist(err) {
			file, info, err = openWebFile(root, "/index.html")
		}
		if err != nil {
			if os.IsPermission(err) {
				http.Error(w, "Forbidden", http.StatusForbidden)
			} else {
				http.NotFound(w, r)
			}
			return
		}
		defer file.Close()
		// ServeContent supports MIME detection, HEAD, ranges and conditional GET,
		// without redirecting SPA routes to the physical index.html filename.
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	})
}

func openWebFile(root http.Dir, name string) (http.File, fs.FileInfo, error) {
	// http.Dir confines cleaned URL paths to the configured root. Refuse symlink
	// components too: http.Dir alone would follow links outside that root.
	current := string(root)
	for _, part := range strings.Split(strings.TrimPrefix(name, "/"), "/") {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, nil, os.ErrPermission
		}
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		file.Close()
		if err == nil {
			err = os.ErrNotExist // SPA fallback, never a directory listing.
		}
		return nil, nil, err
	}
	return file, info, nil
}
