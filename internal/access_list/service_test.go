package access_list

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type serviceEnv struct {
	dir   string
	tests int
	fail  bool
}

func setupService(t *testing.T) *serviceEnv {
	t.Helper()
	env := &serviceEnv{dir: useConfDir(t)}

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AccessList{}))
	model.Use(db)
	query.SetDefault(db)

	previous := testAndReload
	testAndReload = func(tx *config.FileTransaction) error {
		env.tests++
		if env.fail {
			// Mirror FileTransaction.TestAndReload, which restores the files
			// before it reports a rejected configuration.
			return config.RollbackError(errors.New("emerg: unexpected"), tx.Rollback)
		}
		return nil
	}
	t.Cleanup(func() {
		testAndReload = previous
		model.Use(nil)
	})
	return env
}

func (env *serviceEnv) read(t *testing.T, slug string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(env.dir, "nginx-ui", "access", slug+".conf"))
	require.NoError(t, err)
	return string(content)
}

func (env *serviceEnv) writeSite(t *testing.T, name, content string) {
	t.Helper()
	path := filepath.Join(env.dir, "sites-available", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func newList(name, slug string, rules ...model.AccessRule) *model.AccessList {
	return &model.AccessList{Name: name, Slug: slug, Fallback: model.AccessFallbackDeny, Rules: rules}
}

func allow(v string) model.AccessRule { return model.AccessRule{Type: model.AccessRuleAllow, Value: v} }

func TestSaveUnusedListSkipsNginx(t *testing.T) {
	env := setupService(t)

	result, err := Save(newList("LAN", "lan", allow("192.168.1.0/24")))
	require.NoError(t, err)
	assert.NotZero(t, result.List.ID)
	assert.Equal(t, []string{"lan"}, result.Slugs)
	assert.Empty(t, result.References)
	assert.Contains(t, env.read(t, "lan"), "allow 192.168.1.0/24;\ndeny all;\n")
	assert.Zero(t, env.tests, "a file nothing includes needs no nginx test")
}

func TestSaveRerendersDependentsAndKeepsSlug(t *testing.T) {
	env := setupService(t)

	lan, err := Save(newList("LAN", "lan", allow("192.168.1.0/24")))
	require.NoError(t, err)
	_, err = Save(newList("Office", "office", model.AccessRule{Type: model.AccessRuleRef, RefID: lan.List.ID}))
	require.NoError(t, err)
	env.writeSite(t, "nas", "server {\n    include nginx-ui/access/office.conf;\n}\n")

	changed := newList("LAN renamed", "attempted-rename", allow("10.0.0.0/8"))
	changed.ID = lan.List.ID
	result, err := Save(changed)
	require.NoError(t, err)

	assert.Equal(t, "lan", result.List.Slug, "the slug never changes")
	assert.ElementsMatch(t, []string{"lan", "office"}, result.Slugs)
	require.Len(t, result.References, 1)
	assert.Equal(t, "nas", result.References[0].Name)
	assert.Equal(t, 1, env.tests)
	assert.Contains(t, env.read(t, "office"), "# >>> LAN renamed (lan)\nallow 10.0.0.0/8;\n")
	_, err = os.Stat(filepath.Join(env.dir, "nginx-ui", "access", "attempted-rename.conf"))
	assert.True(t, os.IsNotExist(err))
}

func TestSaveRollsBackWhenNginxRejects(t *testing.T) {
	env := setupService(t)

	lan, err := Save(newList("LAN", "lan", allow("192.168.1.0/24")))
	require.NoError(t, err)
	before := env.read(t, "lan")
	env.writeSite(t, "nas", "server {\n    include nginx-ui/access/lan.conf;\n}\n")

	env.fail = true
	changed := newList("LAN", "lan", allow("10.0.0.0/8"))
	changed.ID = lan.List.ID
	_, err = Save(changed)
	require.Error(t, err)
	assertErrCode(t, err, ErrNginxTestFailed)

	assert.Equal(t, before, env.read(t, "lan"), "the file is restored")
	stored, err := Get(lan.List.ID)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.0/24", stored.Rules[0].Value, "the database change is rolled back")
}

func TestSaveRejectsCycle(t *testing.T) {
	setupService(t)

	a, err := Save(newList("A", "a", allow("::1")))
	require.NoError(t, err)
	_, err = Save(newList("B", "b", model.AccessRule{Type: model.AccessRuleRef, RefID: a.List.ID}))
	require.NoError(t, err)

	b, err := All()
	require.NoError(t, err)
	var bID uint64
	for _, l := range b {
		if l.Slug == "b" {
			bID = l.ID
		}
	}
	changed := newList("A", "a", model.AccessRule{Type: model.AccessRuleRef, RefID: bID})
	changed.ID = a.List.ID
	_, err = Save(changed)
	assertErrCode(t, err, ErrReferenceCycle)
}

func TestDeleteGuardsReferences(t *testing.T) {
	env := setupService(t)

	lan, err := Save(newList("LAN", "lan", allow("::1")))
	require.NoError(t, err)
	office, err := Save(newList("Office", "office", model.AccessRule{Type: model.AccessRuleRef, RefID: lan.List.ID}))
	require.NoError(t, err)

	err = Delete(lan.List.ID)
	assertErrCode(t, err, ErrAccessListInUse)
	assert.Contains(t, err.Error(), "Office")

	env.writeSite(t, "nas", "server {\n    location / {\n        include nginx-ui/access/office.conf;\n    }\n}\n")
	err = Delete(office.List.ID)
	assertErrCode(t, err, ErrAccessListInUse)
	assert.Contains(t, err.Error(), "site nas (location /)")

	usage, err := UsageOf(office.List.ID)
	require.NoError(t, err)
	assert.Len(t, usage.References, 1)
	assert.Empty(t, usage.Lists)

	env.writeSite(t, "nas", "server {\n}\n")
	require.NoError(t, Delete(office.List.ID))
	_, err = os.Stat(filepath.Join(env.dir, "nginx-ui", "access", "office.conf"))
	assert.True(t, os.IsNotExist(err))
	require.NoError(t, Delete(lan.List.ID))

	// The slug can be used again after a delete.
	_, err = Save(newList("LAN again", "lan", allow("::1")))
	require.NoError(t, err)
}

func TestSlugsExist(t *testing.T) {
	setupService(t)
	_, err := Save(newList("LAN", "lan", allow("::1")))
	require.NoError(t, err)

	require.NoError(t, SlugsExist([]string{"lan"}))
	assertErrCode(t, SlugsExist([]string{"lan", "nope"}), ErrUnknownList)
}
