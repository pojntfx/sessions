package locales

import "embed"

//go:generate sh -c "find . -type f -name *.po | parallel --progress msgfmt -o {.}.mo {}"
//go:embed *
var FS embed.FS
