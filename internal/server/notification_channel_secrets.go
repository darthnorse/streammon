package server

import (
	"encoding/json"

	"streammon/internal/models"
)

// maskSecret returns maskedSecret when v is non-empty, otherwise the empty
// string, so absent secrets aren't rendered as configured.
func maskSecret(v string) string {
	if v == "" {
		return v
	}
	return maskedSecret
}

// unmaskSecret returns the previously stored value when v is the
// maskedSecret placeholder (meaning the client left the field unchanged),
// otherwise v unchanged.
func unmaskSecret(v, stored string) string {
	if v == maskedSecret {
		return stored
	}
	return v
}

// maskChannelConfig returns a copy of raw with secret fields (Discord
// webhook URL, webhook auth headers, Pushover API token, Ntfy token)
// replaced by maskedSecret, so secrets never leave the server in cleartext.
// raw itself is never mutated; on any decode error raw is returned unchanged
// so callers still see a validation error rather than a silently-empty config.
func maskChannelConfig(ct models.ChannelType, raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}

	switch ct {
	case models.ChannelTypeDiscord:
		var cfg models.DiscordConfig
		if json.Unmarshal(raw, &cfg) != nil {
			return raw
		}
		cfg.WebhookURL = maskSecret(cfg.WebhookURL)
		return marshalOrFallback(cfg, raw)

	case models.ChannelTypeWebhook:
		var cfg models.WebhookConfig
		if json.Unmarshal(raw, &cfg) != nil {
			return raw
		}
		if len(cfg.Headers) > 0 {
			masked := make(map[string]string, len(cfg.Headers))
			for k, v := range cfg.Headers {
				masked[k] = maskSecret(v)
			}
			cfg.Headers = masked
		}
		return marshalOrFallback(cfg, raw)

	case models.ChannelTypePushover:
		var cfg models.PushoverConfig
		if json.Unmarshal(raw, &cfg) != nil {
			return raw
		}
		cfg.APIToken = maskSecret(cfg.APIToken)
		return marshalOrFallback(cfg, raw)

	case models.ChannelTypeNtfy:
		var cfg models.NtfyConfig
		if json.Unmarshal(raw, &cfg) != nil {
			return raw
		}
		cfg.Token = maskSecret(cfg.Token)
		return marshalOrFallback(cfg, raw)

	default:
		return raw
	}
}

// restoreChannelSecrets replaces any maskedSecret placeholder left in newRaw
// (meaning the client didn't change that field) with the corresponding value
// from existingRaw, so a PUT that echoes back a masked response never
// clobbers the stored secret with the literal "********" string.
func restoreChannelSecrets(ct models.ChannelType, newRaw, existingRaw json.RawMessage) json.RawMessage {
	if len(newRaw) == 0 || len(existingRaw) == 0 {
		return newRaw
	}

	switch ct {
	case models.ChannelTypeDiscord:
		var newCfg, oldCfg models.DiscordConfig
		if json.Unmarshal(newRaw, &newCfg) != nil {
			return newRaw
		}
		_ = json.Unmarshal(existingRaw, &oldCfg)
		newCfg.WebhookURL = unmaskSecret(newCfg.WebhookURL, oldCfg.WebhookURL)
		return marshalOrFallback(newCfg, newRaw)

	case models.ChannelTypeWebhook:
		var newCfg, oldCfg models.WebhookConfig
		if json.Unmarshal(newRaw, &newCfg) != nil {
			return newRaw
		}
		_ = json.Unmarshal(existingRaw, &oldCfg)
		for k, v := range newCfg.Headers {
			newCfg.Headers[k] = unmaskSecret(v, oldCfg.Headers[k])
		}
		return marshalOrFallback(newCfg, newRaw)

	case models.ChannelTypePushover:
		var newCfg, oldCfg models.PushoverConfig
		if json.Unmarshal(newRaw, &newCfg) != nil {
			return newRaw
		}
		_ = json.Unmarshal(existingRaw, &oldCfg)
		newCfg.APIToken = unmaskSecret(newCfg.APIToken, oldCfg.APIToken)
		return marshalOrFallback(newCfg, newRaw)

	case models.ChannelTypeNtfy:
		var newCfg, oldCfg models.NtfyConfig
		if json.Unmarshal(newRaw, &newCfg) != nil {
			return newRaw
		}
		_ = json.Unmarshal(existingRaw, &oldCfg)
		newCfg.Token = unmaskSecret(newCfg.Token, oldCfg.Token)
		return marshalOrFallback(newCfg, newRaw)

	default:
		return newRaw
	}
}

// maskChannel returns a copy of c with its Config secrets masked. c itself
// (and its underlying Config bytes) is left untouched.
func maskChannel(c models.NotificationChannel) models.NotificationChannel {
	c.Config = maskChannelConfig(c.ChannelType, c.Config)
	return c
}

func maskChannels(cs []models.NotificationChannel) []models.NotificationChannel {
	masked := make([]models.NotificationChannel, len(cs))
	for i, c := range cs {
		masked[i] = maskChannel(c)
	}
	return masked
}

func marshalOrFallback(v any, fallback json.RawMessage) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return fallback
	}
	return b
}
