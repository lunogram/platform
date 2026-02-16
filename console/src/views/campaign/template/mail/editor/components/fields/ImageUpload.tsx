/* eslint-disable react-hooks/rules-of-hooks */
import { usePuck, type CustomField } from "@puckeditor/core";
import { getViewportTailwindBreakpoint } from "../../viewport";
import { useTranslation } from "react-i18next";
import { ImageIcon, Upload, Trash2, Loader2 } from "lucide-react";
import { useContext, useState } from "react";
import { ProjectContext } from "@/contexts";
import api from "@/api";

import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";

export interface ImageUploadViewport {
  url?: string;
  size?: "cover" | "contain" | "auto";
  repeat?: "no-repeat" | "repeat";
}

export interface ImageUploadProps {
  sm?: Partial<ImageUploadViewport>;
  md?: Partial<ImageUploadViewport>;
  xl?: Partial<ImageUploadViewport>;
}

const maxBreakpointWidth: number = 1280;

export const ImageUpload: CustomField<ImageUploadProps> = {
  type: "custom",
  render: ({ onChange, value = {} }) => {
    const { t } = useTranslation();
    const { appState } = usePuck();
    const [project] = useContext(ProjectContext);
    const [isUploading, setIsUploading] = useState(false);

    const viewport = appState.ui.viewports.current;
    const breakpoint = getViewportTailwindBreakpoint(
      typeof viewport.width === "number" ? viewport.width : maxBreakpointWidth,
    );

    const config = value[breakpoint] || {};

    const handleChange = (field: keyof ImageUploadViewport, val: string) => {
      onChange({
        ...value,
        [breakpoint]: { ...config, [field]: val },
      });
    };

    const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file || !project?.id) return;

      setIsUploading(true);
      try {
        const createResponse = await api.images.create(project.id, file);
        const response = await api.images.get(
          project.id,
          createResponse.documents[0],
        );
        const blob = response;

        const reader = new FileReader();
        reader.onloadend = () => {
          const base64data = reader.result as string;
          handleChange("url", base64data);
        };
        reader.readAsDataURL(blob);
      } catch (error) {
        console.error("Upload failed:", error);
        alert("Failed to upload image.");
      } finally {
        setIsUploading(false);
      }
    };

    const handleRemove = () => {
      onChange({ ...value, [breakpoint]: {} });
    };

    const hasImage = !!config.url;

    return (
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <label className="text-xs font-semibold text-gray-700 uppercase tracking-wide">
            {t("campaign.template.editor.components.image.title")}
          </label>
          {hasImage && !isUploading && (
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 text-red-500 hover:text-red-700"
              onClick={handleRemove}
            >
              <Trash2 className="h-3 w-3" />
            </Button>
          )}
        </div>

        {isUploading ? (
          <div className="flex flex-col items-center justify-center w-full h-24 border-2 border-dashed border-indigo-200 bg-indigo-50 rounded-lg">
            <Loader2 className="w-6 h-6 text-indigo-500 animate-spin mb-2" />
            <p className="text-[10px] text-indigo-600 font-medium">
              Uploading to project...
            </p>
          </div>
        ) : !hasImage ? (
          <div className="space-y-2">
            <label className="flex flex-col items-center justify-center w-full h-24 border-2 border-dashed border-gray-300 rounded-lg cursor-pointer hover:bg-gray-50 transition-colors">
              <div className="flex flex-col items-center justify-center pt-5 pb-6">
                <Upload className="w-6 h-6 text-gray-400 mb-2" />
                <p className="text-[10px] text-gray-500">
                  {t("campaign.template.editor.components.image.upload_hint")}
                </p>
              </div>
              <input
                type="file"
                className="hidden"
                accept="image/*"
                onChange={handleFileChange}
              />
            </label>
            <div className="relative">
              <Input
                placeholder="Paste direct image URL..."
                className="h-8 text-xs pr-8"
                onBlur={(e) => handleChange("url", e.target.value)}
              />
              <ImageIcon className="absolute right-2 top-2 h-4 w-4 text-gray-400" />
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {/* Image Preview */}
            <div
              className="w-full h-32 rounded-lg bg-center bg-no-repeat border border-gray-200 relative group"
              style={{
                backgroundImage: `url(${config.url})`,
                backgroundSize: "contain",
                backgroundColor: "#f9fafb",
              }}
            >
              <div className="absolute inset-0 bg-black/20 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center rounded-lg">
                <label className="cursor-pointer bg-white p-2 rounded-full shadow-md">
                  <Upload className="h-4 w-4 text-gray-700" />
                  <input
                    type="file"
                    className="hidden"
                    accept="image/*"
                    onChange={handleFileChange}
                  />
                </label>
              </div>
            </div>

            {/* Settings */}
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <label className="text-[10px] font-medium text-gray-500 uppercase">
                  {t("editor.fields.image.size")}
                </label>
                <Select
                  value={config.size ?? "cover"}
                  onValueChange={(v) => handleChange("size", v)}
                >
                  <SelectTrigger className="h-7 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="cover">Cover</SelectItem>
                    <SelectItem value="contain">Contain</SelectItem>
                    <SelectItem value="auto">Auto</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <label className="text-[10px] font-medium text-gray-500 uppercase">
                  {t("editor.fields.image.repeat")}
                </label>
                <Select
                  value={config.repeat ?? "no-repeat"}
                  onValueChange={(v) => handleChange("repeat", v)}
                >
                  <SelectTrigger className="h-7 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="no-repeat">No Repeat</SelectItem>
                    <SelectItem value="repeat">Repeat</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>
        )}
      </div>
    );
  },
};
