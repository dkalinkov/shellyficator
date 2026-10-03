package accounts

import "github.com/pocketbase/pocketbase/core"

// CreateVerified creates an account within the caller's transaction.
func CreateVerified(app core.App, collectionName, email, password string) error {
	collection, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.SetEmail(email)
	record.SetPassword(password)
	record.SetVerified(true)
	return app.Save(record)
}
