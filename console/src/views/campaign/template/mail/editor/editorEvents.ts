// Event bus for communication between plugin and preview without React state
// Exposed on window so iframe can access it via window.parent

type HtmlChangeListener = (html: string) => void;
type ActiveChangeListener = (active: boolean) => void;

declare global {
    interface Window {
        __editorEvents?: EditorEventBus;
    }
}

class EditorEventBus {
    private html = "";
    private active = false;
    private htmlListeners = new Set<HtmlChangeListener>();
    private activeListeners = new Set<ActiveChangeListener>();

    setHtml(html: string) {
        if (this.html === html) return;
        this.html = html;
        this.htmlListeners.forEach((listener) => listener(html));
    }

    getHtml() {
        return this.html;
    }

    setActive(active: boolean) {
        if (this.active === active) return;
        this.active = active;
        this.activeListeners.forEach((listener) => listener(active));
    }

    getActive() {
        return this.active;
    }

    subscribeHtml(listener: HtmlChangeListener) {
        this.htmlListeners.add(listener);
        return () => {
            this.htmlListeners.delete(listener);
        };
    }

    subscribeActive(listener: ActiveChangeListener) {
        this.activeListeners.add(listener);
        return () => {
            this.activeListeners.delete(listener);
        };
    }

    reset() {
        this.html = "";
        this.active = false;
    }
}

function getEditorEvents(): EditorEventBus {
    if (typeof window !== "undefined" && window.parent !== window) {
        if (window.parent.__editorEvents) {
            return window.parent.__editorEvents;
        }
    }

    if (typeof window !== "undefined") {
        if (!window.__editorEvents) {
            window.__editorEvents = new EditorEventBus();
        }
        return window.__editorEvents;
    }

    return new EditorEventBus();
}

export const editorEvents = getEditorEvents();

export const rawHtmlState = {
    get active() {
        return editorEvents.getActive();
    },
    get html() {
        return editorEvents.getHtml();
    },
};
