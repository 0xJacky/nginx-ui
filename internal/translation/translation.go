package translation

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/0xJacky/Nginx-UI/app"
	"github.com/0xJacky/pofile"
	"github.com/samber/lo"
)

var Dict map[string]pofile.Dict

func init() {
	Dict = make(map[string]pofile.Dict)

	fs, err := app.GetDistFS()
	if err != nil {
		log.Fatalln("Failed to get DistFS:", err)
	}

	i18nJson, err := fs.Open("i18n.json")
	if err != nil {
		log.Fatalln("Failed to open i18n.json:", err)
	}

	defer i18nJson.Close()

	bytes, _ := io.ReadAll(i18nJson)

	i18nMap := make(map[string]string)

	_ = json.Unmarshal(bytes, &i18nMap)

	langCode := lo.MapToSlice(i18nMap, func(key string, value string) string {
		return key
	})

	for _, v := range langCode {
		handlePo(v)
	}
}

func handlePo(langCode string) {
	fsys, err := app.GetDistFS()
	if err != nil {
		log.Fatalln("Failed to get DistFS:", err)
	}

	file, err := fsys.Open(fmt.Sprintf("src/language/%s/app.po", langCode))

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
