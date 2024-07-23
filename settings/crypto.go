package settings

import "crypto/md5"

type Crypto struct {
	Secret string
}

var CryptoSettings = Crypto{}

func (c *Crypto) GetSecretMd5() []byte {
	k := md5.Sum([]byte(c.Secret))
	return k[:]
}
