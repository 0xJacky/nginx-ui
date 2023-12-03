package settings

type Casdoor struct {
	Endpoint     string `json:"endpoint"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Certificate  string `json:"certificate"`
	Organization string `json:"organization"`
	Application  string `json:"application"`
	RedirectUri  string `json:"redirect_uri"`
}

var CasdoorSettings = Casdoor{
	Endpoint:     "",
	ClientId:     "",
	ClientSecret: "",
	Certificate:  "",
	Organization: "",
	Application:  "",
	RedirectUri:  "",
}
