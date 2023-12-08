package translation

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/app"
	"github.com/0xJacky/pofile/pofile"
	"io"
	"log"
)

var Dict map[string]pofile.Dict

func init() {
	Dict = make(map[string]pofile.Dict)

	langCode := []string{"zh_CN", "zh_TW", "ru_RU", "fr_FR", "es", "vi_VN"}

	for _, v := range langCode {
		handlePo(v)
	}
}

func handlePo(langCode string) {
	file, err := app.DistFS.Open(fmt.Sprintf("src/language/%s/app.po", langCode))

	if err != nil {
		log.Fatalln(err)
	}

	defer file.Close()

	bytes, err := io.ReadAll(file)

	if err != nil {
		log.Fatalln(err)
	}

	p, err := pofile.ParseText(string(bytes))

	if err != nil {
		log.Fatalln(err)
	}

	Dict[langCode] = p.ToDict()
}

func GetTranslation(langCode string) pofile.Dict {
	return Dict[langCode]
}
