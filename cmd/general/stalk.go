package general

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gowa/internal/command"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func init() {
	command.Register(&command.Command{
		Name:    "stalk",
		Cmd:     "stalk",
		Categ:   "general",
		Desc:    "Stalk profil WhatsApp lengkap (Info, LID, Bio, Devices, Bisnis)",
		Alias:   []string{"infor", "whois", "stalkwa", "profile", "infowa"},
		Execute: handleStalk,
	})
}

func handleStalk(ctx *command.Context) error {
	ctx.Typing()

	c := ctx.Ctx
	if c == nil {
		c = context.Background()
	}

	var (
		targetJID      types.JID
		targetPN       types.JID
		targetLID      types.JID
		targetPushName string
	)

	ci := ctx.GetContextInfo()

	if len(ctx.Args) > 0 {
		if ci != nil && len(ci.GetMentionedJID()) > 0 {
			for _, m := range ci.GetMentionedJID() {
				if parsed, err := types.ParseJID(m); err == nil && !parsed.IsEmpty() {
					targetJID = parsed.ToNonAD()
					break
				}
			}
		}

		if targetJID.IsEmpty() {
			raw := strings.TrimSpace(ctx.RawArgs)
			if strings.Contains(raw, "@lid") {
				if parsed, err := types.ParseJID(raw); err == nil && !parsed.IsEmpty() {
					targetJID = parsed.ToNonAD()
					targetLID = targetJID
				}
			} else if strings.Contains(raw, "@s.whatsapp.net") {
				if parsed, err := types.ParseJID(raw); err == nil && !parsed.IsEmpty() {
					targetJID = parsed.ToNonAD()
					targetPN = targetJID
				}
			} else {
				num := cleanNumber(raw)
				if len(num) >= 8 {
					chkCtx, chkCancel := context.WithTimeout(c, 15*time.Second)
					defer chkCancel()

					resp, err := ctx.Client.IsOnWhatsApp(chkCtx, []string{"+" + num})
					if err != nil {
						return ctx.Reply("❌ Gagal memeriksa nomor di WhatsApp: " + err.Error())
					}
					if len(resp) == 0 || !resp[0].IsIn {
						return ctx.Reply(fmt.Sprintf("❌ Nomor +%s tidak terdaftar di WhatsApp.", num))
					}
					if !resp[0].JID.IsEmpty() {
						targetPN = resp[0].JID.ToNonAD()
					} else {
						targetPN = types.NewJID(num, types.DefaultUserServer)
					}
					targetJID = targetPN
				} else {
					return ctx.Reply(fmt.Sprintf("❌ Format nomor tidak valid.\n\n*Cara Penggunaan:*\n- Reply pesan target: `%sstalk`\n- Tag pengguna: `%sstalk @user`\n- Masukkan nomor: `%sstalk 628xxxxxxxx`\n- Profil sendiri: `%sstalk`", ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix))
				}
			}
		}
	} else if ci != nil && ci.GetQuotedMessage() != nil {
		pStr := ci.GetParticipant()
		if pStr != "" {
			if parsed, err := types.ParseJID(pStr); err == nil && !parsed.IsEmpty() {
				targetJID = parsed.ToNonAD()
			}
		} else if !ctx.IsGroup {
			targetJID = ctx.Chat.ToNonAD()
		}
	}

	if targetJID.IsEmpty() {
		targetJID = ctx.Sender.ToNonAD()
		if !ctx.SenderPN.IsEmpty() {
			targetPN = ctx.SenderPN.ToNonAD()
		}
		targetPushName = ctx.Event.Info.PushName
	}

	if targetJID.Server == types.HiddenUserServer {
		targetLID = targetJID
		if targetPN.IsEmpty() {
			if pn, err := ctx.ResolvePN(targetLID); err == nil && !pn.IsEmpty() {
				targetPN = pn.ToNonAD()
			}
		}
	} else if targetJID.Server == types.DefaultUserServer {
		targetPN = targetJID
		if targetLID.IsEmpty() {
			if lid, err := ctx.ResolveLID(targetPN); err == nil && !lid.IsEmpty() {
				targetLID = lid.ToNonAD()
			}
		}
	}

	queryJID := targetJID
	if !targetPN.IsEmpty() {
		queryJID = targetPN
	}

	var (
		wg          sync.WaitGroup
		userInfo    *types.UserInfo
		bizProfile  *types.BusinessProfile
		groupInfo   *types.GroupInfo
		picInfo     *types.ProfilePictureInfo
		contactInfo types.ContactInfo
	)

	var commonGID types.JID
	if ctx.IsGroup {
		commonGID = ctx.Chat.ToNonAD()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		qCtx, qCancel := context.WithTimeout(c, 15*time.Second)
		defer qCancel()

		infoMap, err := ctx.Client.GetUserInfo(qCtx, []types.JID{queryJID})
		if err == nil && infoMap != nil {
			if u, ok := infoMap[queryJID]; ok {
				userInfo = &u
			} else if !targetPN.IsEmpty() {
				if u, ok := infoMap[targetPN]; ok {
					userInfo = &u
				}
			} else if !targetLID.IsEmpty() {
				if u, ok := infoMap[targetLID]; ok {
					userInfo = &u
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		bCtx, bCancel := context.WithTimeout(c, 10*time.Second)
		defer bCancel()

		biz, err := ctx.Client.GetBusinessProfile(bCtx, queryJID)
		if err == nil && biz != nil {
			bizProfile = biz
		} else if !targetPN.IsEmpty() && targetPN != queryJID {
			if biz, err := ctx.Client.GetBusinessProfile(bCtx, targetPN); err == nil && biz != nil {
				bizProfile = biz
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		reqCtx, reqCancel := context.WithTimeout(c, 15*time.Second)
		defer reqCancel()

		fetchPic := func(jid types.JID) *types.ProfilePictureInfo {
			info, err := ctx.Client.GetProfilePictureInfo(reqCtx, jid, &whatsmeow.GetProfilePictureParams{
				Preview:   false,
				CommonGID: commonGID,
			})
			if err == nil && info != nil && info.URL != "" {
				return info
			}
			info, err = ctx.Client.GetProfilePictureInfo(reqCtx, jid, &whatsmeow.GetProfilePictureParams{
				Preview:   true,
				CommonGID: commonGID,
			})
			if err == nil && info != nil && info.URL != "" {
				return info
			}
			return nil
		}

		pic := fetchPic(queryJID)
		if pic == nil && !targetPN.IsEmpty() && targetPN != queryJID {
			pic = fetchPic(targetPN)
		}
		if pic == nil && !targetLID.IsEmpty() && targetLID != queryJID {
			pic = fetchPic(targetLID)
		}
		picInfo = pic
	}()

	if ctx.IsGroup {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gCtx, gCancel := context.WithTimeout(c, 10*time.Second)
			defer gCancel()

			g, err := ctx.Client.GetGroupInfo(gCtx, ctx.Chat)
			if err == nil && g != nil {
				groupInfo = g
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if ctx.Client.Store != nil && ctx.Client.Store.Contacts != nil {
			cCtx, cCancel := context.WithTimeout(c, 5*time.Second)
			defer cCancel()

			ct, err := ctx.Client.Store.Contacts.GetContact(cCtx, queryJID)
			if err == nil && ct.Found {
				contactInfo = ct
			} else if !targetPN.IsEmpty() {
				if ct, err := ctx.Client.Store.Contacts.GetContact(cCtx, targetPN); err == nil && ct.Found {
					contactInfo = ct
				}
			}
		}
	}()

	wg.Wait()

	if userInfo != nil {
		if targetLID.IsEmpty() && !userInfo.LID.IsEmpty() {
			targetLID = userInfo.LID.ToNonAD()
		}
	}

	var (
		inGroup      bool
		isSuperAdmin bool
		isAdmin      bool
		groupName    string
	)

	if groupInfo != nil {
		groupName = groupInfo.GroupName.Name
		for _, p := range groupInfo.Participants {
			match := false
			if !targetPN.IsEmpty() && (p.PhoneNumber == targetPN || p.JID == targetPN) {
				match = true
			} else if !targetLID.IsEmpty() && (p.LID == targetLID || p.JID == targetLID) {
				match = true
			} else if !targetJID.IsEmpty() && (p.JID == targetJID || p.PhoneNumber == targetJID || p.LID == targetJID) {
				match = true
			}

			if match {
				inGroup = true
				isSuperAdmin = p.IsSuperAdmin
				isAdmin = p.IsAdmin
				if targetPN.IsEmpty() && !p.PhoneNumber.IsEmpty() {
					targetPN = p.PhoneNumber.ToNonAD()
				}
				if targetLID.IsEmpty() && !p.LID.IsEmpty() {
					targetLID = p.LID.ToNonAD()
				}
				if targetPushName == "" && p.DisplayName != "" {
					targetPushName = p.DisplayName
				}
				if !targetLID.IsEmpty() && !targetPN.IsEmpty() {
					go ctx.Client.StoreLIDPNMapping(context.Background(), targetLID, targetPN)
				}
				break
			}
		}
	}

	if targetPushName == "" && contactInfo.PushName != "" {
		targetPushName = contactInfo.PushName
	}
	if targetPushName == "" && userInfo != nil && userInfo.VerifiedName != nil && userInfo.VerifiedName.Details != nil {
		targetPushName = userInfo.VerifiedName.Details.GetVerifiedName()
	}

	targetNumber := ""
	if !targetPN.IsEmpty() {
		targetNumber = targetPN.User
	} else if targetJID.Server == types.DefaultUserServer {
		targetNumber = targetJID.User
	}

	var devices []types.JID
	if userInfo != nil && len(userInfo.Devices) > 0 {
		devices = userInfo.Devices
	} else {
		devCtx, devCancel := context.WithTimeout(c, 5*time.Second)
		defer devCancel()
		if devs, err := ctx.Client.GetUserDevicesContext(devCtx, []types.JID{queryJID}); err == nil {
			devices = devs
		}
	}

	var imgData []byte
	if picInfo != nil && picInfo.URL != "" {
		dlCtx, dlCancel := context.WithTimeout(c, 15*time.Second)
		defer dlCancel()
		req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, picInfo.URL, nil)
		if err == nil {
			httpResp, err := http.DefaultClient.Do(req)
			if err == nil {
				defer httpResp.Body.Close()
				if httpResp.StatusCode == http.StatusOK {
					if data, err := io.ReadAll(httpResp.Body); err == nil && len(data) > 0 {
						imgData = data
					}
				}
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("*WhatsApp Profile Info*\n\n")

	if targetNumber != "" {
		sb.WriteString(fmt.Sprintf("- *Tag:* @%s\n", targetNumber))
	} else if !targetLID.IsEmpty() {
		sb.WriteString(fmt.Sprintf("- *Tag LID:* @%s\n", targetLID.User))
	} else {
		sb.WriteString(fmt.Sprintf("- *Tag JID:* @%s\n", targetJID.User))
	}

	if targetPushName != "" {
		sb.WriteString(fmt.Sprintf("- *Push Name:* %s\n", targetPushName))
	}

	if contactInfo.FullName != "" {
		sb.WriteString(fmt.Sprintf("- *Nama Kontak:* %s\n", contactInfo.FullName))
	}

	if targetNumber != "" {
		sb.WriteString(fmt.Sprintf("- *Nomor HP:* %s\n", formatStalkNumber(targetNumber)))
		sb.WriteString(fmt.Sprintf("- *JID Phone:* %s@s.whatsapp.net\n", targetNumber))
	}

	if !targetLID.IsEmpty() {
		sb.WriteString(fmt.Sprintf("- *JID LID:* %s\n", targetLID.String()))
	}

	isBusiness := bizProfile != nil || (userInfo != nil && userInfo.VerifiedName != nil)
	isVerified := userInfo != nil && userInfo.VerifiedName != nil
	isBot := targetJID.IsBot() || (!targetPN.IsEmpty() && targetPN.IsBot())

	accountType := "WhatsApp Standar (Personal)"
	if isVerified {
		accountType = "WhatsApp Business (Terverifikasi ✅)"
	} else if isBusiness {
		accountType = "WhatsApp Business"
	}
	sb.WriteString(fmt.Sprintf("- *Tipe Akun:* %s\n", accountType))

	if isBot {
		sb.WriteString("- *Status Bot:* Ya (Official Bot)\n")
	}

	userBio := ""
	if userInfo != nil {
		userBio = strings.TrimSpace(userInfo.Status)
	}
	if userBio != "" {
		sb.WriteString(fmt.Sprintf("- *Bio / About:* %s\n", userBio))
	}

	if targetNumber != "" {
		country, flag := getStalkCountry(targetNumber)
		operator := getStalkOperator(targetNumber)
		sb.WriteString(fmt.Sprintf("- *Negara:* %s %s\n", flag, country))
		if operator != "" {
			sb.WriteString(fmt.Sprintf("- *Provider:* %s\n", operator))
		}
		sb.WriteString(fmt.Sprintf("- *Direct Chat:* https://wa.me/%s\n", targetNumber))
	}

	if len(devices) > 0 {
		primaryCount := 0
		companionCount := 0
		var devDetails []string
		for _, dev := range devices {
			if dev.Device == 0 {
				primaryCount++
				devDetails = append(devDetails, "Ponsel Utama (#0)")
			} else {
				companionCount++
				devDetails = append(devDetails, fmt.Sprintf("Web/Desktop (#%d)", dev.Device))
			}
		}
		sb.WriteString(fmt.Sprintf("- *Multi-Device:* %d Perangkat Aktif (%d Ponsel Utama, %d Web/Desktop)\n", len(devices), primaryCount, companionCount))
		if len(devDetails) > 0 {
			sb.WriteString(fmt.Sprintf("- *Sesi Perangkat:* %s\n", strings.Join(devDetails, ", ")))
		}
	}

	if ctx.IsGroup {
		roleStr := "Anggota"
		if isSuperAdmin {
			roleStr = "👑 Pembuat Grup / Super Admin"
		} else if isAdmin {
			roleStr = "🛡️ Admin Grup"
		} else if !inGroup {
			roleStr = "Bukan Anggota Grup"
		}

		if groupName != "" {
			sb.WriteString(fmt.Sprintf("- *Grup:* %s (Jabatan: %s)\n", groupName, roleStr))
		} else {
			sb.WriteString(fmt.Sprintf("- *Jabatan Grup:* %s\n", roleStr))
		}
	}

	if isBusiness && bizProfile != nil {
		sb.WriteString("\n*Profil Bisnis*\n")
		if len(bizProfile.Categories) > 0 {
			var catNames []string
			for _, cat := range bizProfile.Categories {
				if cat.Name != "" {
					catNames = append(catNames, cat.Name)
				}
			}
			if len(catNames) > 0 {
				sb.WriteString(fmt.Sprintf("- *Kategori:* %s\n", strings.Join(catNames, ", ")))
			}
		}

		if bizProfile.ProfileOptions != nil {
			if desc, ok := bizProfile.ProfileOptions["description"]; ok && strings.TrimSpace(desc) != "" {
				sb.WriteString(fmt.Sprintf("- *Deskripsi:* %s\n", strings.TrimSpace(desc)))
			}
		}

		if bizProfile.Email != "" {
			sb.WriteString(fmt.Sprintf("- *Email:* %s\n", bizProfile.Email))
		}
		if bizProfile.Address != "" {
			sb.WriteString(fmt.Sprintf("- *Alamat:* %s\n", bizProfile.Address))
		}

		if bizProfile.ProfileOptions != nil {
			var sites []string
			for k, v := range bizProfile.ProfileOptions {
				val := strings.TrimSpace(v)
				if val == "" {
					continue
				}
				if strings.Contains(strings.ToLower(k), "website") || strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
					sites = append(sites, val)
				}
			}
			if len(sites) > 0 {
				sort.Strings(sites)
				sb.WriteString(fmt.Sprintf("- *Website:* %s\n", strings.Join(sites, ", ")))
			}
		}

		if bizProfile.BusinessHoursTimeZone != "" {
			sb.WriteString(fmt.Sprintf("- *Zona Waktu:* %s\n", bizProfile.BusinessHoursTimeZone))
		}
	}

	reportText := strings.TrimSpace(sb.String())

	mentionJID := ""
	if targetNumber != "" {
		mentionJID = targetNumber + "@s.whatsapp.net"
	} else if !targetLID.IsEmpty() {
		mentionJID = targetLID.String()
	} else {
		mentionJID = targetJID.String()
	}

	if len(imgData) > 0 {
		return ctx.ReplyImage(imgData, "image/jpeg", reportText, mentionJID)
	}

	return ctx.ReplyWithMentions(reportText, mentionJID)
}

func formatStalkNumber(num string) string {
	if strings.HasPrefix(num, "62") && len(num) >= 10 {
		if len(num) == 10 {
			return fmt.Sprintf("+62 %s-%s", num[2:5], num[5:])
		} else if len(num) == 11 {
			return fmt.Sprintf("+62 %s-%s-%s", num[2:5], num[5:8], num[8:])
		} else if len(num) == 12 {
			return fmt.Sprintf("+62 %s-%s-%s", num[2:5], num[5:9], num[9:])
		} else if len(num) >= 13 {
			return fmt.Sprintf("+62 %s-%s-%s", num[2:5], num[5:9], num[9:])
		}
	}
	return "+" + num
}

func getStalkOperator(num string) string {
	if !strings.HasPrefix(num, "62") || len(num) < 5 {
		return ""
	}
	prefix4 := num[:5]
	switch prefix4 {
	case "62811", "62812", "62813", "62821", "62822", "62823", "62851", "62852", "62853":
		return "Telkomsel (by.U / simPATI / Halo)"
	case "62814", "62815", "62816", "62855", "62856", "62857", "62858":
		return "Indosat Ooredoo Hutchison (IM3)"
	case "62817", "62818", "62819", "62859", "62877", "62878":
		return "XL Axiata"
	case "62831", "62832", "62833", "62838":
		return "AXIS (XL Axiata)"
	case "62895", "62896", "62897", "62898", "62899":
		return "Tri / 3 (IOH)"
	case "62881", "62882", "62883", "62884", "62885", "62886", "62887", "62888", "62889":
		return "Smartfren"
	case "62828":
		return "Net1 Indonesia"
	default:
		return "Provider Seluler Indonesia"
	}
}

func getStalkCountry(num string) (country string, flag string) {
	if strings.HasPrefix(num, "62") {
		return "Indonesia (+62)", "🇮🇩"
	} else if strings.HasPrefix(num, "60") {
		return "Malaysia (+60)", "🇲🇾"
	} else if strings.HasPrefix(num, "65") {
		return "Singapura (+65)", "🇸🇬"
	} else if strings.HasPrefix(num, "66") {
		return "Thailand (+66)", "🇹🇭"
	} else if strings.HasPrefix(num, "63") {
		return "Filipina (+63)", "🇵🇭"
	} else if strings.HasPrefix(num, "1") {
		return "Amerika Serikat / Kanada (+1)", "🇺🇸"
	} else if strings.HasPrefix(num, "44") {
		return "Inggris Raya (+44)", "🇬🇧"
	} else if strings.HasPrefix(num, "81") {
		return "Jepang (+81)", "🇯🇵"
	} else if strings.HasPrefix(num, "82") {
		return "Korea Selatan (+82)", "🇰🇷"
	} else if strings.HasPrefix(num, "86") {
		return "China (+86)", "🇨🇳"
	} else if strings.HasPrefix(num, "91") {
		return "India (+91)", "🇮🇳"
	} else if strings.HasPrefix(num, "966") {
		return "Arab Saudi (+966)", "🇸🇦"
	} else if strings.HasPrefix(num, "971") {
		return "Uni Emirat Arab (+971)", "🇦🇪"
	} else if strings.HasPrefix(num, "7") {
		return "Rusia / Kazakhstan (+7)", "🇷🇺"
	} else if strings.HasPrefix(num, "61") {
		return "Australia (+61)", "🇦🇺"
	} else if strings.HasPrefix(num, "49") {
		return "Jerman (+49)", "🇩🇪"
	} else if strings.HasPrefix(num, "33") {
		return "Prancis (+33)", "🇫🇷"
	} else if strings.HasPrefix(num, "55") {
		return "Brasil (+55)", "🇧🇷"
	} else if strings.HasPrefix(num, "90") {
		return "Turki (+90)", "🇹🇷"
	}
	return "Internasional", "🌐"
}
