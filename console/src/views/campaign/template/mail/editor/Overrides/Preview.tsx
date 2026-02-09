import parse from "html-react-parser";
import { useContext, useEffect, useState } from "react";
import { TemplateContext } from "@/contexts";
import { editorEvents } from "../editorEvents";

// @ts-expect-error don't worry about it :)
export const Preview = ({ children }) => {
  const [template] = useContext(TemplateContext);
  const initialHtml = template.data.rawHtml ?? "";
  const [rawHtml, setRawHtml] = useState(initialHtml);

  useEffect(() => {
    const currentHtml = editorEvents.getHtml();
    if (currentHtml && currentHtml !== initialHtml) {
      setRawHtml(currentHtml);
    }

    const unsubscribe = editorEvents.subscribeHtml(setRawHtml);
    return unsubscribe;
  }, [initialHtml]);

  if (!rawHtml) {
    return <>{children}</>;
  }

  return <div className="w-full h-full overflow-auto">{parse(rawHtml)}</div>;
};
