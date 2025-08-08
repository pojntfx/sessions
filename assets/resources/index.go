package resources

import (
	_ "embed"
	"path"
	"strings"
)

const (
	AppID      = "com.pojtinger.felicitas.Sessions"
	AppVersion = "0.1.0"
)

//go:generate sh -c "blueprint-compiler batch-compile . . *.blp && glib-compile-resources *.gresource.xml"
//go:embed index.gresource
var ResourceContents []byte

var (
	AppPath = path.Join("/com", "pojtinger", "felicitas", "Sessions")

	AppDevelopers = []string{"Felicitas Pojtinger"}
	AppArtists    = AppDevelopers
	AppCopyright  = "© 2025 " + strings.Join(AppDevelopers, ", ")

	ResourceWindowUIPath = path.Join(AppPath, "window.ui")
	ResourceMetainfoPath = path.Join(AppPath, "metainfo.xml")
)
