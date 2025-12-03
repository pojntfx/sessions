package po

import "embed"

//go:generate sh -c "find .. -name '*.go' -o -name '*.blp' | xgettext --language=C++ --keyword=_ --keyword=L --omit-header -o sessions.pot --files-from=- && find .. -name '*.desktop.in' | xgettext --language=Desktop --keyword=Name --keyword=Comment --omit-header -j -o sessions.pot --files-from=- && find .. -name 'metainfo.xml.in' | xgettext --its=/usr/share/gettext/its/metainfo.its --omit-header -j -o sessions.pot --files-from=-"
//go:generate sh -c "find . -name '*.po' -print0 | xargs -0 -I {} msgmerge --update --backup=none \"{}\" sessions.pot"
//go:generate sh -c "find . -type f -name '*.po' -print0 | xargs -0 -I {} sh -c 'mkdir -p $(basename {} .po)/LC_MESSAGES && msgfmt -o $(basename {} .po)/LC_MESSAGES/sessions.mo {}'"
//go:generate sh -c "find . -name \"*.po\" -exec basename {} .po \\; > LINGUAS"
//go:embed *
var FS embed.FS
