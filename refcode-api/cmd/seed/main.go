// Command seed 建立第一個 admin 帳號，並在本機塞一批 demo 服務商。
// 只給本機開發用，不要在正式環境跑（--demo 會寫入假資料）。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"refcode-api/internal/auth"
	"refcode-api/internal/config"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

func main() {
	var (
		adminEmail    = flag.String("admin-email", "", "要建立的 admin email")
		adminPassword = flag.String("admin-password", "", "admin 密碼（至少 8 個字元）")
		withDemo      = flag.Bool("demo", false, "一併塞入 demo 分類與服務商")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.IsProduction() {
		log.Fatal("seed 只給本機用，APP_ENV=production 時拒絕執行")
	}

	ctx := context.Background()
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	if *adminEmail != "" {
		if err := createAdmin(ctx, st, *adminEmail, *adminPassword); err != nil {
			log.Fatal(err)
		}
	}
	if *withDemo {
		if err := seedDemo(ctx, st); err != nil {
			log.Fatal(err)
		}
	}
	if *adminEmail == "" && !*withDemo {
		flag.Usage()
		os.Exit(1)
	}
}

func createAdmin(ctx context.Context, st *store.Store, email, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	admin, err := st.CreateAdmin(ctx, dbgen.CreateAdminParams{
		Email:        email,
		PasswordHash: hash,
		DisplayName:  email,
		Role:         "owner",
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return fmt.Errorf("admin %s 已經存在", email)
		}
		return err
	}
	fmt.Printf("已建立 admin：%s（role=%s）\n", admin.Email, admin.Role)
	return nil
}

func seedDemo(ctx context.Context, st *store.Store) error {
	categories := []struct{ slug, name string }{
		{"bank", "銀行信用卡"},
		{"invest", "券商投資"},
		{"delivery", "外送"},
		{"streaming", "影音串流"},
	}

	ids := map[string]string{}
	for i, c := range categories {
		cat, err := st.CreateCategory(ctx, dbgen.CreateCategoryParams{
			Slug: c.slug, Name: c.name, SortOrder: int32(i),
		})
		if err != nil {
			if !store.IsUniqueViolation(err) {
				return err
			}
			continue
		}
		ids[c.slug] = cat.ID.String()
		fmt.Printf("分類：%s\n", cat.Name)
	}

	rows, err := st.ListCategories(ctx)
	if err != nil {
		return err
	}
	byslug := map[string]dbgen.MerchantCategory{}
	for _, c := range rows {
		byslug[c.Slug] = c
	}

	merchants := []struct {
		slug, name, category, signupURL, reward, regex string
	}{
		{"demo-bank", "示範銀行", "bank", "https://example.com/signup", "雙方各得 500 元刷卡金", `^[A-Z0-9]{6,10}$`},
		{"demo-broker", "示範證券", "invest", "https://example.com/open", "推薦人得 1 股，被推薦人免手續費 3 個月", ""},
		{"demo-delivery", "示範外送", "delivery", "https://example.com/ref", "雙方各得 100 元折扣", ""},
		{"demo-stream", "示範影音", "streaming", "https://example.com/invite", "被推薦人首月免費", ""},
	}

	for _, m := range merchants {
		cat, ok := byslug[m.category]
		if !ok {
			continue
		}
		var regex *string
		if m.regex != "" {
			regex = &m.regex
		}
		created, err := st.CreateMerchant(ctx, dbgen.CreateMerchantParams{
			Slug:            m.slug,
			Name:            m.name,
			CategoryID:      cat.ID,
			SignupUrl:       m.signupURL,
			RewardDesc:      m.reward,
			CodeFormatRegex: regex,
		})
		if err != nil {
			if !store.IsUniqueViolation(err) {
				return err
			}
			continue
		}
		fmt.Printf("服務商：%s（%s）\n", created.Name, created.Slug)
	}
	return nil
}
