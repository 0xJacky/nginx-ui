package frontend

import (
	"embed"
	"github.com/0xJacky/pofile"
	"log"
	"path"
)

//go:embed dist
var DistFS embed.FS

var Translations pofile.Dict

func InitTranslations() {
	lang := []string{"zh_CN", "zh_TW", "en"}
	Translations = make(pofile.Dict)
	for _, v := range lang {
		p, err := pofile.Parse(path.Join("frontend", "src", "locale", v, "LC_MESSAGES", "app.po"))
		if err != nil {
			log.Fatalln(err)
		}
		Translations[p.Header.Language] = make(pofile.Dict)
		Translations[p.Header.Language] = p.ToDict()
	}
}
