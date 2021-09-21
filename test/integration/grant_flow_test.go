package integration

import (
	"context"
	"flag"
	"os"
	"strings"
	"testing"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kemics/ebay"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

var (
	integration  bool
	clientID     string
	clientSecret string
	redirectURL  string
)

func init() {
	flag.BoolVar(&integration, "integration", false, "run integration tests")
	flag.Parse()
	if !integration {
		return
	}
	clientID = os.Getenv("SANDBOX_CLIENT_ID")
	clientSecret = os.Getenv("SANDBOX_CLIENT_SECRET")

	// Your accept redirect URL should be setup to redirect to https://localhost:52125/accept
	redirectURL = os.Getenv("SANDBOX_RU_NAME")

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		panic("Please set SANDBOX_CLIENT_ID, SANDBOX_CLIENT_SECRET and SANDBOX_REDIRECT_URL.")
	}
}

// TestGrantFlows is a verbose integration test that checks the client credentials grant flow as well as the
// authorization code grant flow are working properly on the eBay sandbox.
// Make sure to set the various environment variables required.
func TestGrantFlows(t *testing.T) {
	if !integration {
		t.SkipNow()
	}

	// You have to manually create an auction in the sandbox and retrieve its URL.
	// Auctions can't be created using the rest api (yet?).
	auctionURL := os.Getenv("SANDOX_AUCTION_URL")

	ctx := context.Background()

	conf := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     ebay.OAuth20SandboxEndpoint.TokenURL,
		Scopes:       []string{ebay.ScopeRoot},
	}

	client := ebay.NewSandboxClient(oauth2.NewClient(ctx, ebay.TokenSource(conf.TokenSource(ctx))))

	lit, err := client.Buy.Browse.GetItemByLegacyID(ctx, auctionURL[strings.LastIndex(auctionURL, "/")+1:])
	if err != nil {
		t.Fatalf("%+v", err)
	}
	it, err := client.Buy.Browse.GetItem(ctx, lit.ItemID)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	t.Logf("Item ID is %q", it.ItemID)
}
