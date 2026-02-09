import debounceCompile from "../../compiler/debounceCompile";
import type { EditorChangeListener, EditorCode } from "../shared/types";
import { codeStore } from "../CodeEditorPlugins/codeStore";
import { useEditorRef } from "../CodeEditorPlugins/editorRef";

export default class editorEvents {
    private static instance: editorEvents;
    private events: Map<string, EditorChangeListener> = new Map();

    public static getInstance(): editorEvents {
        if (!editorEvents.instance) {
            editorEvents.instance = new editorEvents();
        }
        return editorEvents.instance;
    }

    constructor() {
        this.registerEventListener("htmlChanged", () => {
            debounceCompile.getInstance().debounce(() => {
                const html = useEditorRef().current;
                codeStore.setCode(html);
            }, 500)
        })
    }

    public registerEventListener(eventName: string, callback: EditorChangeListener): void {
        this.events.set(eventName, callback);
    }

    public unregisterEventListener(eventName: string): void {
        this.events.delete(eventName);
    }

    public fireEvent(eventName: string, html: EditorCode): void {
        const callback = this.events.get(eventName);
        if (callback) {
            callback(html);
        }
    }
}