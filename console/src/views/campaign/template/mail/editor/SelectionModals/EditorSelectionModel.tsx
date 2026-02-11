import React from "react";
import { Layout, Code2 } from "lucide-react";
import { ChoiceCard } from "./ChoiceCard";

interface EditorSelectionModalProps {
  isOpen: boolean;
  onSelect: (type: "block" | "code") => void;
}

export const EditorSelectionModal: React.FC<EditorSelectionModalProps> = ({
  isOpen,
  onSelect,
}) => {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-9999 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="relative w-full max-w-xl overflow-hidden rounded-2xl bg-white shadow-2xl transition-all">
        <div className="flex items-start justify-between border-b border-gray-100 px-8 py-6">
          <div>
            <h2 className="text-2xl font-bold text-gray-900">New Project</h2>
            <p className="mt-1 text-sm text-gray-500">
              Select your preferred editing experience.
            </p>
          </div>
        </div>

        <div className="p-8">
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <ChoiceCard
              title="Visual Builder"
              description="Drag & drop blocks to design visually."
              icon={<Layout size={32} />}
              onClick={() => onSelect("block")}
            />
            <ChoiceCard
              title="Developer Mode"
              description="Write raw HTML for full control."
              icon={<Code2 size={32} />}
              onClick={() => onSelect("code")}
            />
          </div>
        </div>

        <div className="bg-gray-50 px-8 py-4 border-t border-gray-100">
          <p className="text-[11px] uppercase tracking-wider font-semibold text-gray-400">
            Note: You can <strong>not</strong> toggle between these modes inside
            the editor.
          </p>
        </div>
      </div>
    </div>
  );
};
