package settings

type Casdoor struct {
	Endpoint     string `json:"endpoint" protected:"true"`
	ClientId     string `json:"client_id" protected:"true"`
	ClientSecret string `json:"client_secret" protected:"true"`
	Certificate  string `json:"certificate" protected:"true"`
	Organization string `json:"organization" protected:"true"`
	Application  string `json:"application" protected:"true"`
	RedirectUri  string `json:"redirect_uri" protected:"true"`
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
