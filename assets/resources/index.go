package resources

import (
	_ "embed"
	"path"
	"strings"
)

//go:generate sh -c "find ../../po -name '*.po' | sed 's|^\\../../po/||; s|\\.po$||' > ../../po/LINGUAS"
//go:generate sh -c "msgfmt --desktop --template ../../assets/meta/com.pojtinger.felicitas.Sessions.desktop.in -d ../../po -o - -f | sed 's|/LC_MESSAGES/default||g' > ../../assets/meta/com.pojtinger.felicitas.Sessions.desktop"
//go:generate sh -c "msgfmt --xml -L metainfo --template ../../assets/resources/metainfo.xml.in -d ../../po -o - -f | sed 's|/LC[-_]MESSAGES/default||g' > ../../assets/resources/metainfo.xml"

const (
	AppID      = "com.pojtinger.felicitas.Sessions"
	AppVersion = "0.1.2"
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
