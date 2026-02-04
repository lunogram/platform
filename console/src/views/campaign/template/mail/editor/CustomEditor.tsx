import { Puck } from "@puckeditor/core";

export function CustomEditor() {
  return (
    <Puck
      config={}
      data={}
      overrides={{
        header: () => null,
        headerActions: () => null,
        actionBar: () => null,
      }}
    >
      {/* 
         Puck's default UI is now gone. 
         You can build any layout you want here.
      */}
      <div className="my-custom-layout-grid">
        <aside className="left-sidebar">
          <h2>Drag these:</h2>
          <Puck.Components />
        </aside>

        <main className="editor-canvas">
          <Puck.Preview />
        </main>

        <aside className="right-sidebar">
          <h2>Editing:</h2>
          <Puck.Fields />
        </aside>
      </div>
    </Puck>
  );
}
