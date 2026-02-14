package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	kratos "github.com/ory/client-go"
)

func main() {
	email := flag.String("email", "", "Email of the user to promote")
	role := flag.String("role", "admin", "Role to assign (default: admin)")
	adminURL := flag.String("admin-url", "http://localhost:4434", "Kratos Admin API URL")
	flag.Parse()

	if *email == "" {
		log.Fatal("Please provide an email using --email")
	}

	configuration := kratos.NewConfiguration()
	configuration.Servers = kratos.ServerConfigurations{
		{
			URL: *adminURL,
		},
	}
	apiClient := kratos.NewAPIClient(configuration)

	// List identities to find the user by email
	// Note: Kratos ListIdentities doesn't natively support filtering by trait in all versions cleanly via SDK in one go,
	// but we can list and filter or use the experimental search if enabled.
	// For simplicity and standard compatibility, we'll list and filter (assuming reasonable number of users or using pagination if needed, though CLI is for ad-hoc).
	// Better approach for production with many users: Use Kratos `ListIdentities` with `CredentialsIdentifier` if available or just iterate.
	// Actually, Kratos Admin API allows listing identities. We'll list and look for the email.

	// In a real prod env with millions of users, you'd want to use the Identity ID directly or a lookup service.
	// For this tool, we'll iterate pages to find the user.

	ctx := context.Background()

	var targetIdentity *kratos.Identity
	page := int64(0)
	perPage := int64(100)

	fmt.Printf("Searching for user with email: %s...\n", *email)

	for {
		identities, _, err := apiClient.IdentityAPI.ListIdentities(ctx).Page(page).PerPage(perPage).Execute()
		if err != nil {
			log.Fatalf("Error listing identities: %v", err)
		}

		if len(identities) == 0 {
			break
		}

		for i := range identities {
			id := identities[i]
			traits, ok := id.Traits.(map[string]interface{})
			if !ok {
				continue
			}

			if traitEmail, ok := traits["email"].(string); ok && traitEmail == *email {
				targetIdentity = &id
				break
			}
		}

		if targetIdentity != nil {
			break
		}
		page++
	}

	if targetIdentity == nil {
		log.Fatalf("User with email %s not found", *email)
	}

	fmt.Printf("Found user: %s (ID: %s)\n", *email, targetIdentity.Id)
	fmt.Printf("Current Role: %v\n", getRole(targetIdentity.Traits))

	// Update traits
	traits, _ := targetIdentity.Traits.(map[string]interface{})
	traits["role"] = *role

	// Create patch
	// We need to use PatchIdentity
	// The SDK PatchIdentity requires a JsonPatchDocument.
	// However, it's often easier to use UpdateIdentity if we have the full object, but Patch is preferred for partial updates.
	// Let's use UpdateIdentity for simplicity as we have the full identity and just want to modify traits.

	updateParams := *kratos.NewUpdateIdentityBody(
		targetIdentity.SchemaId,
		*targetIdentity.State,
		traits,
	)

	updatedIdentity, _, err := apiClient.IdentityAPI.UpdateIdentity(ctx, targetIdentity.Id).UpdateIdentityBody(updateParams).Execute()
	if err != nil {
		log.Fatalf("Error updating identity: %v\nFull error: %+v", err, err)
	}

	fmt.Printf("Successfully promoted %s to %s\n", *email, *role)
	fmt.Printf("New Role: %v\n", getRole(updatedIdentity.Traits))
}

func getRole(traits interface{}) string {
	t, ok := traits.(map[string]interface{})
	if !ok {
		return "unknown"
	}
	if role, ok := t["role"].(string); ok {
		return role
	}
	return "none"
}
