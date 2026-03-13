import { useEffect, useRef, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { AlertCircle } from "lucide-react"

/**
 * Script injected into the iframe to enable element selection.
 *
 * Selection targets, in priority order:
 * 1. Elements with an explicit [data-section] attribute (ideal case).
 * 2. Top-level <table> elements inside the email body that represent
 *    React Email <Section> components (the common compiled output).
 *
 * Supports multi-select: clicking an element toggles its selection state.
 * The label shown to the user is derived from `data-section`, or
 * auto-generated from the element's text content / tag name.
 */
const SELECTOR_SCRIPT = `
<script>
(function() {
    var selectorEnabled = false;
    var hoveredEl = null;
    var selectedEls = [];
    /* Track elements whose overflow we temporarily changed so we can restore. */
    var overflowRestoreList = [];

    /**
     * Temporarily set overflow:visible on the element and every ancestor up to
     * <body> that has overflow:hidden (or clip). Store originals for restore.
     */
    function forceOverflowVisible(el) {
        var cur = el;
        while (cur && cur !== document.documentElement) {
            var cs = window.getComputedStyle(cur);
            if (cs.overflow !== 'visible' || cs.overflowX !== 'visible' || cs.overflowY !== 'visible') {
                overflowRestoreList.push({ el: cur, ov: cur.style.overflow });
                cur.style.overflow = 'visible';
            }
            cur = cur.parentElement;
        }
    }

    function restoreOverflow() {
        for (var i = 0; i < overflowRestoreList.length; i++) {
            overflowRestoreList[i].el.style.overflow = overflowRestoreList[i].ov;
        }
        overflowRestoreList = [];
    }

    function setHighlight(el, on) {
        if (!el) return;
        if (on === 'hover') {
            el.style.position = 'relative';
            el.style.zIndex = '999';
            el.style.boxShadow = '0 0 0 2px rgba(59,130,246,0.45), 0 0 10px 0 rgba(59,130,246,0.2)';
            el.style.borderRadius = '4px';
            forceOverflowVisible(el);
        } else if (on === 'select') {
            el.style.position = 'relative';
            el.style.zIndex = '999';
            el.style.boxShadow = '0 0 0 2px rgba(59,130,246,0.6), 0 0 14px 2px rgba(59,130,246,0.3)';
            el.style.borderRadius = '4px';
            forceOverflowVisible(el);
        } else {
            el.style.position = '';
            el.style.zIndex = '';
            el.style.boxShadow = '';
            el.style.borderRadius = '';
            restoreOverflow();
        }
    }

    function isSelected(el) {
        for (var i = 0; i < selectedEls.length; i++) {
            if (selectedEls[i] === el) return true;
        }
        return false;
    }

    function clearHover() {
        if (hoveredEl && !isSelected(hoveredEl)) {
            setHighlight(hoveredEl, false);
        }
        hoveredEl = null;
    }

    function clearAllSelected() {
        for (var i = 0; i < selectedEls.length; i++) {
            setHighlight(selectedEls[i], false);
        }
        selectedEls = [];
    }

    /* Walk up from target to find the best selectable ancestor. */
    function findSection(el) {
        /* First: look for an explicit data-section attribute */
        var cur = el;
        while (cur && cur !== document.body && cur !== document.documentElement) {
            if (cur.hasAttribute && cur.hasAttribute('data-section')) return cur;
            cur = cur.parentElement;
        }

        /* Fallback: find the nearest top-level table (React Email Section).
           We want tables that are direct children of a <td> inside the
           outermost container, OR direct children of <body>.
           Heuristic: walk up until we find a <table> whose parent is <td>
           or <body>. */
        cur = el;
        while (cur && cur !== document.body && cur !== document.documentElement) {
            if (cur.tagName === 'TABLE') {
                var p = cur.parentElement;
                if (!p || p.tagName === 'BODY' || p.tagName === 'TD' || p.tagName === 'DIV') {
                    return cur;
                }
            }
            cur = cur.parentElement;
        }
        return null;
    }

    /* Derive a human-readable label for the element. */
    function getLabel(el) {
        if (el.hasAttribute('data-section')) return el.getAttribute('data-section');
        /* Grab first meaningful text (heading, first text node, alt text) */
        var heading = el.querySelector('h1,h2,h3,h4,h5,h6');
        if (heading) return heading.textContent.trim().slice(0, 60);
        var img = el.querySelector('img[alt]');
        if (img && img.alt) return 'Image: ' + img.alt.trim().slice(0, 50);
        var btn = el.querySelector('a[href]');
        if (btn && btn.textContent.trim()) return 'Button: ' + btn.textContent.trim().slice(0, 50);
        var txt = (el.textContent || '').trim().slice(0, 60);
        return txt || el.tagName.toLowerCase();
    }

    document.addEventListener('mousemove', function(e) {
        if (!selectorEnabled) return;
        var section = findSection(e.target);
        if (section === hoveredEl) return;
        clearHover();
        if (section) {
            hoveredEl = section;
            if (!isSelected(section)) {
                setHighlight(hoveredEl, 'hover');
            }
        }
    });

    document.addEventListener('click', function(e) {
        if (!selectorEnabled) return;
        var section = findSection(e.target);
        if (section) {
            e.preventDefault();
            e.stopPropagation();
            var sectionId = section.hasAttribute('data-section') ? section.getAttribute('data-section') : undefined;
            var rawText = (section.textContent || '').replace(/\\s+/g, ' ').trim().slice(0, 200);
            var label = getLabel(section);

            if (isSelected(section)) {
                /* Deselect: remove from list */
                selectedEls = selectedEls.filter(function(el) { return el !== section; });
                window.parent.postMessage({
                    type: 'SEC_DESEL',
                    label: label
                }, '*');
            } else {
                /* Select: add to list, clear the hover highlight */
                selectedEls.push(section);
                setHighlight(section, false);
                window.parent.postMessage({
                    type: 'SEC_SEL',
                    label: label,
                    sectionId: sectionId,
                    textContent: rawText || undefined
                }, '*');
            }
        }
    }, true);

    document.addEventListener('mouseleave', function() {
        clearHover();
    });

    window.addEventListener('message', function(e) {
        if (e.data === 'SEL_ON') {
            selectorEnabled = true;
        } else if (e.data === 'SEL_OFF') {
            selectorEnabled = false;
            clearHover();
            clearAllSelected();
        }
    });
})();
</script>
`

interface EmailPreviewPanelProps {
    html: string
    error: string | null
    viewport: string
    viewportWidth: number
    /** Called when a section is selected in the preview */
    onSectionSelect?: (section: { label: string; sectionId?: string; textContent?: string }) => void
    /** Called when an already-selected section is deselected */
    onSectionDeselect?: (label: string) => void
    /** Whether the element selector is currently active */
    selectorActive?: boolean
}

export function EmailPreviewPanel({
    html,
    error,
    viewportWidth,
    onSectionSelect,
    onSectionDeselect,
    selectorActive = false,
}: EmailPreviewPanelProps) {
    const { t } = useTranslation()
    const iframeRef = useRef<HTMLIFrameElement>(null)

    // Always inject selector script into the HTML so the iframe document stays
    // stable when toggling between code and builder mode. The script is inert
    // until it receives a "SEL_ON" message, so it has no visible effect when
    // selection is not active.
    const preparedHtml = (() => {
        if (!html) return html
        if (html.includes("</body>")) {
            return html.replace("</body>", `${SELECTOR_SCRIPT}</body>`)
        }
        if (html.includes("</html>")) {
            return html.replace("</html>", `${SELECTOR_SCRIPT}</html>`)
        }
        return html + SELECTOR_SCRIPT
    })()

    // Toggle selector mode in the iframe based on selectorActive
    useEffect(() => {
        const iframe = iframeRef.current
        if (!iframe?.contentWindow) return

        const sendToggle = () => {
            iframe.contentWindow?.postMessage(selectorActive ? "SEL_ON" : "SEL_OFF", "*")
        }

        sendToggle()
        iframe.addEventListener("load", sendToggle)
        return () => {
            iframe.removeEventListener("load", sendToggle)
        }
    }, [selectorActive, preparedHtml])

    // Listen for section selection/deselection messages from the iframe
    const handleMessage = useCallback(
        (e: MessageEvent) => {
            if (!e.data || typeof e.data !== "object") return

            if (e.data.type === "SEC_SEL" && onSectionSelect) {
                onSectionSelect({
                    label: e.data.label as string,
                    sectionId: e.data.sectionId as string | undefined,
                    textContent: e.data.textContent as string | undefined,
                })
            } else if (e.data.type === "SEC_DESEL" && onSectionDeselect) {
                onSectionDeselect(e.data.label as string)
            }
        },
        [onSectionSelect, onSectionDeselect],
    )

    useEffect(() => {
        window.addEventListener("message", handleMessage)
        return () => window.removeEventListener("message", handleMessage)
    }, [handleMessage])

    if (error) {
        return (
            <div className="flex-1 min-h-0 overflow-hidden h-full flex items-start justify-center p-6">
                <div className="max-w-md w-full rounded-lg border border-destructive/30 bg-destructive/5 p-4">
                    <div className="flex items-start gap-3">
                        <AlertCircle className="h-4 w-4 text-destructive mt-0.5 shrink-0" />
                        <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium text-destructive">
                                {t(
                                    "campaign.template.email.editor.compileError",
                                    "Compilation Error",
                                )}
                            </p>
                            <pre className="mt-2 text-xs text-destructive/80 whitespace-pre-wrap break-words font-mono leading-relaxed">
                                {error}
                            </pre>
                        </div>
                    </div>
                </div>
            </div>
        )
    }

    if (!html) {
        return (
            <div className="flex-1 min-h-0 overflow-hidden h-full flex items-center justify-center">
                <p className="text-sm text-muted-foreground">
                    {t(
                        "campaign.template.email.editor.noPreview",
                        "Start typing to see a preview...",
                    )}
                </p>
            </div>
        )
    }

    return (
        <div className="flex-1 min-h-0 min-w-0 overflow-hidden h-full px-6 py-4 flex justify-center">
            <iframe
                ref={iframeRef}
                srcDoc={preparedHtml}
                title="Email preview"
                className="border rounded-md bg-white shadow-sm h-full min-w-0"
                style={{
                    width: "100%",
                    maxWidth: viewportWidth,
                }}
                sandbox="allow-same-origin allow-scripts"
            />
        </div>
    )
}
