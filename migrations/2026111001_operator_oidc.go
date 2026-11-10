package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		operators, err := app.FindCollectionByNameOrId("operators")
		if err != nil {
			return err
		}
		operators.Fields.Add(&core.TextField{Name: "oidc_issuer", Hidden: true, Max: 2048}, &core.TextField{Name: "oidc_subject", Hidden: true, Max: 512})
		operators.AddIndex("idx_operator_oidc_identity", true, "oidc_issuer, oidc_subject", "oidc_issuer != '' AND oidc_subject != ''")
		return app.Save(operators)
	}, func(app core.App) error {
		operators, err := app.FindCollectionByNameOrId("operators")
		if err != nil {
			return err
		}
		operators.RemoveIndex("idx_operator_oidc_identity")
		operators.Fields.RemoveByName("oidc_issuer")
		operators.Fields.RemoveByName("oidc_subject")
		return app.Save(operators)
	})
}
