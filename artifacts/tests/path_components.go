package main

import (
	"path/filepath"
)

// PathObject structure
type PathObject struct {
	filesystem FileSystem
	name       string
	path       string
	obj        interface{}
}

func (p *PathObject) IsDirectory() bool {
	return p.filesystem.IsDirectory(p)
}

func (p *PathObject) IsFile() bool {
	return p.filesystem.IsFile(p)
}

func (p *PathObject) IsSymlink() bool {
	return p.filesystem.IsSymlink(p)
}

func (p *PathObject) ListDirectory() []*PathObject {
	return p.filesystem.ListDirectory(p)
}

// GetPath(): возвращает сам путь текущего объекта (например, "/home/user/file.txt").
// GetChild(path string): принимает относительный путь (например, "subdir") и возвращает новый PathObject, представляющий путь, полученный путем объединения текущего пути и указанного относительного пути.
func (p *PathObject) GetChild(path string) *PathObject {
	return p.filesystem.GetPath(p, path)
}

func (p *PathObject) GetPath() string {
	return p.path
}

func (p *PathObject) ReadChunks() ([][]byte, error) {
	return p.filesystem.ReadChunks(p)
}

func (p *PathObject) GetSize() int64 {
	return p.filesystem.GetSize(p)
}

// PathComponent interface
type PathComponent interface {
	Generate() <-chan *PathObject
}

// RecursionPathComponent structure
type RecursionPathComponent struct {
	directory bool
	maxDepth  int
	source    <-chan *PathObject
}

func NewRecursionPathComponent(directory bool, maxDepth int, source <-chan *PathObject) *RecursionPathComponent {
	return &RecursionPathComponent{
		directory: directory,
		maxDepth:  maxDepth,
		source:    source,
	}
}

func (r *RecursionPathComponent) Generate() <-chan *PathObject {
	out := make(chan *PathObject)
	go func() {
		defer close(out)
		for parent := range r.source {
			r.recurseFromDir(parent, 0, out)
		}
	}()
	return out
}

func (r *RecursionPathComponent) recurseFromDir(parent *PathObject, depth int, out chan<- *PathObject) {
	if depth < r.maxDepth || r.maxDepth == -1 {
		for _, path := range parent.ListDirectory() {
			if path.IsDirectory() {
				r.recurseFromDir(path, depth+1, out)
				if r.directory || path.IsFile() {
					out <- path
				}
			} else if !r.directory {
				out <- path
			}
		}
	}
}

// GlobPathComponent structure
type GlobPathComponent struct {
	directory bool
	pattern   string
	source    <-chan *PathObject
}

func NewGlobPathComponent(directory bool, pattern string, source <-chan *PathObject) *GlobPathComponent {
	return &GlobPathComponent{
		directory: directory,
		pattern:   pattern,
		source:    source,
	}
}

func (g *GlobPathComponent) Generate() <-chan *PathObject {
	out := make(chan *PathObject)
	go func() {
		defer close(out)
		for parent := range g.source {
			for _, path := range parent.ListDirectory() {
				match, _ := filepath.Match(g.pattern, path.name)
				if match {
					if g.directory && path.IsDirectory() {
						out <- path
					} else if !g.directory && path.IsFile() {
						out <- path
					}
				}
			}
		}
	}()
	return out
}

// RegularPathComponent structure
type RegularPathComponent struct {
	directory bool
	path      string
	source    <-chan *PathObject
}

func NewRegularPathComponent(directory bool, path string, source <-chan *PathObject) *RegularPathComponent {
	return &RegularPathComponent{
		directory: directory,
		path:      path,
		source:    source,
	}
}

func (r *RegularPathComponent) Generate() <-chan *PathObject {
	out := make(chan *PathObject)
	go func() {
		defer close(out)
		for parent := range r.source {
			path := parent.GetChild(r.path)
			if path != nil {
				if r.directory && path.IsDirectory() {
					out <- path
				} else if !r.directory && path.IsFile() {
					out <- path
				}
			}
		}
	}()
	return out
}
