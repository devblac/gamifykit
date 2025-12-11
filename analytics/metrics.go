package analytics

import "gamifykit/core"

// BridgeHook bridges an event source to multiple hooks.
type BridgeHook struct{ hooks []Hook }

func NewBridge(hooks ...Hook) *BridgeHook { return &BridgeHook{hooks: hooks} }

func (b *BridgeHook) OnEvent(e core.Event) {
	for _, h := range b.hooks {
		h.OnEvent(e)
	}
}
