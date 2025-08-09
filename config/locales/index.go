package locales

import "embed"

//go:generate sh -c "find ../.. -name '*.go' -o -name '*.blp' | xgettext --language=C++ --keyword=_ --keyword=gcore.Local --keyword=Local --omit-header -o default.pot --files-from=-"
//go:generate sh -c "find . -name 'default.po' -print0 | xargs -0 -I {} msgmerge --update \"{}\" default.pot"
//go:generate sh -c "find . -type f -name '*.po' -print0 | xargs -0 -I {} sh -c 'msgfmt -o \"{}.mo\" \"{}\"' && find . -type f -name '*.po.mo' -exec sh -c 'mv \"{}\" \"$(echo \"{}\" | sed s/\\.po\\.mo/.mo/)\"' \\;"
//go:embed *
var FS embed.FS
