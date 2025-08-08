package locales

import "embed"

//go:generate sh -c "find . -type f -name '*.po' -print0 | xargs -0 -I {} sh -c 'msgfmt -o \"{}.mo\" \"{}\"' && find . -type f -name '*.po.mo' -exec sh -c 'mv \"{}\" \"$(echo \"{}\" | sed s/\\.po\\.mo/.mo/)\"' \\;"
//go:embed *
var FS embed.FS
