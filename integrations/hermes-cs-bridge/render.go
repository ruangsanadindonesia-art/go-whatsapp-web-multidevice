package main

import (
	"regexp"
	"strings"
)

// renderedReply is an AI reply after the bridge replaced its markers.
type renderedReply struct {
	Text        string
	Images      []renderedImage
	FunnelStage string // discovery or rekomendasi; never shown to the lead
	FunnelPackage string
	Handoff     bool
	Reason      string
	PaymentSent bool
	FormSent    bool
	// JadwalKode is the schedule the calon santri chose ([[JADWAL:<kode>]]);
	// the bridge checks it against the open schedules before recording it.
	JadwalKode string
	// Package code and price of the last [[REKENING]] (for the Dashboard CRM).
	PaymentPaket, PaymentNominal string
	Warnings                     []string // unknown codes etc., for the log
}

// renderedImage binds the selected, approved gallery item to a contextual
// WhatsApp caption. When the model omits or invalidates it, use the gallery caption.
type renderedImage struct {
	MediaItem
	Caption string
}

var (
	captionPriceRe = regexp.MustCompile(`(?i)rp\.?\s*[0-9][0-9.,]*`)
	stageRe = regexp.MustCompile(`(?i)\[\[\s*TAHAP\s*:\s*(discovery|rekomendasi)(?:\s*:\s*([a-z0-9_-]+))?\s*\]\]`)
	formRe     = regexp.MustCompile(`(?i)\[\[\s*FORM\s*\]\]`)
	jadwalRe   = regexp.MustCompile(`(?i)\[\[\s*JADWAL\s*:\s*([a-z0-9-]{2,40})\s*\]\]`)
	rekeningRe = regexp.MustCompile(`(?i)\[\[\s*REKENING\s*(?::\s*([^\]\s]*)\s*)?\]\]`)
	gambarRe   = regexp.MustCompile(`(?i)\[\[\s*GAMBAR\s*:\s*([a-z0-9_-]{1,32})(?:\s*\|\s*([^\]\r\n]*))?\s*\]\]`)
	anyMarkRe  = regexp.MustCompile(`\[\[[^\]]{0,200}\]\]`)
	blankRunRe = regexp.MustCompile(`\n{3,}`)
)

const maxImagesPerReply = 2

// renderReply turns the AI text into what is actually sent: official form and
// payment text are pasted from settings (so the model never types an account
// number), images are collected, and any other [[...]] marker is removed.
func renderReply(reply string, s Settings) renderedReply {
	var r renderedReply
	clean, reason, handoff := parseHandoff(reply)
	r.Handoff, r.Reason = handoff, reason

	// Explicit AI milestones are removed before sending. Dashboard changes
	// stage only after the corresponding WhatsApp message succeeds.
	clean = stageRe.ReplaceAllStringFunc(clean, func(m string) string {
		parts := stageRe.FindStringSubmatch(m)
		stage := strings.ToLower(parts[1])
		code := strings.ToLower(parts[2])
		if stage == "discovery" && code == "" {
			if r.FunnelStage != "rekomendasi" {
				r.FunnelStage, r.FunnelPackage = stage, ""
			}
		} else if stage == "rekomendasi" && code != "" {
			if _, valid := s.Package(code); valid {
				r.FunnelStage, r.FunnelPackage = stage, code
			} else {
				r.Warnings = append(r.Warnings, "unknown recommendation package: "+code)
			}
		} else {
			r.Warnings = append(r.Warnings, "invalid funnel stage marker")
		}
		return ""
	})

	// Schedule choice: internal marker, never shown. The last one wins.
	clean = jadwalRe.ReplaceAllStringFunc(clean, func(m string) string {
		r.JadwalKode = strings.ToLower(jadwalRe.FindStringSubmatch(m)[1])
		return ""
	})

	// Markdown cleanup first, so the official templates are not altered.
	clean = toWhatsAppMarkdown(clean)

	if formRe.MatchString(clean) {
		r.FormSent = true
		clean = formRe.ReplaceAllLiteralString(clean, "\n"+strings.TrimSpace(s.FormText)+"\n")
	}

	clean = rekeningRe.ReplaceAllStringFunc(clean, func(m string) string {
		code := ""
		if sm := rekeningRe.FindStringSubmatch(m); len(sm) > 1 {
			code = strings.TrimSpace(sm[1])
		}
		paket, nominal := "sesuai pilihan Kakak", "akan dikonfirmasi admin"
		if p, ok := s.Package(code); ok {
			paket, nominal = p.Name, p.Price
			r.PaymentPaket, r.PaymentNominal = strings.ToLower(p.Code), p.Price
		} else {
			r.Warnings = append(r.Warnings, "unknown package code in [[REKENING]]: "+code)
		}
		r.PaymentSent = true
		txt := strings.ReplaceAll(strings.TrimSpace(s.PaymentText), "{PAKET}", paket)
		return "\n" + strings.ReplaceAll(txt, "{NOMINAL}", nominal) + "\n"
	})

	seen := map[string]bool{}
	clean = gambarRe.ReplaceAllStringFunc(clean, func(m string) string {
		parts := gambarRe.FindStringSubmatch(m)
		code := strings.ToLower(parts[1])
		item, ok := s.MediaByCode(code)
		switch {
		case !ok:
			r.Warnings = append(r.Warnings, "unknown image code: "+code)
		case seen[code]:
		case len(r.Images) >= maxImagesPerReply:
			r.Warnings = append(r.Warnings, "image limit reached, skipped: "+code)
		default:
			seen[code] = true
			caption := item.Caption
			if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
				candidate := strings.TrimSpace(parts[2])
				if len([]rune(candidate)) <= 1000 && !strings.Contains(candidate, "[[") && validCaptionPrices(candidate, s) {
					caption = toWhatsAppMarkdown(candidate)
				} else {
					r.Warnings = append(r.Warnings, "invalid contextual image caption: "+code)
				}
			}
			r.Images = append(r.Images, renderedImage{MediaItem: item, Caption: caption})
		}
		return ""
	})

	if leftovers := anyMarkRe.FindAllString(clean, -1); len(leftovers) > 0 {
		r.Warnings = append(r.Warnings, "removed unknown markers: "+strings.Join(leftovers, " "))
		clean = anyMarkRe.ReplaceAllString(clean, "")
	}

	clean = blankRunRe.ReplaceAllString(clean, "\n\n")
	r.Text = strings.TrimSpace(clean)
	return r
}

// validCaptionPrices prevents hallucinated monetary amounts in dynamic captions.
// A price mentioned in a caption must match an official configured package.
func validCaptionPrices(caption string, s Settings) bool {
	for _, mentioned := range captionPriceRe.FindAllString(caption, -1) {
		digits := onlyPriceDigits(mentioned)
		matched := false
		for _, p := range s.Packages {
			if digits != "" && digits == onlyPriceDigits(p.Price) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func onlyPriceDigits(s string) string {
	var b strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}
