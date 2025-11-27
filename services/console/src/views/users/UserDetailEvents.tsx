import { useCallback, useContext, useState } from "react";
import api from "../../api";
import { ProjectContext, UserContext } from "../../contexts";
import { useResolver } from "../../hooks";
import type { SearchParams, UserEvent } from "../../types";
import Modal from "../../ui/Modal";
import { SearchTable } from "../../ui/SearchTable";
import { Column, Columns, JsonPreview } from "../../ui";
import { PreferencesContext } from "../../ui/PreferencesContext";
import { formatDate } from "../../utils";
import Iframe from "../../ui/Iframe";
import { useTranslation } from "react-i18next";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { JsonBuilderField } from "@/components/ui/jsonBuilder";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export default function UserDetailEvents() {
  const { t } = useTranslation();
  const [preferences] = useContext(PreferencesContext);
  const [project] = useContext(ProjectContext);
  const [user] = useContext(UserContext);
  const [params, setParams] = useState<SearchParams>({
    limit: 25,
    q: "",
  });
  const [refreshKey, setRefreshKey] = useState(0);
  const projectId = project.id;
  const userId = user.id;
  const [results] = useResolver(
    useCallback(
      async () => await api.users.events(projectId, userId, params),
      // refreshKey is intentionally included to trigger a refresh when events are added
      // eslint-disable-next-line react-hooks/exhaustive-deps
      [projectId, userId, params, refreshKey]
    )
  );
  const [event, setEvent] = useState<UserEvent>();
  const hasPreview = !!event?.data?.result?.message?.html;
  const [addEventModal, setAddEventModal] = useState(false);
  const [eventData, setEventData] = useState<unknown>({});

  return (
    <>
      <SearchTable
        results={results}
        params={params}
        setParams={setParams}
        title={t("events")}
        itemKey={({ item }) => item.id}
        columns={[
          { key: "name", title: t("name") },
          { key: "created_at", title: t("created_at") },
        ]}
        onSelectRow={setEvent}
        actions={[
          <Button
            key="add_event"
            onClick={() => {
              setAddEventModal(true);
            }}
          >
            {t("add_event")}
          </Button>,
        ]}
      />
      <Dialog open={addEventModal} onOpenChange={() => setAddEventModal(false)}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle className="m-0!">{t("add_event")}</DialogTitle>
            <DialogDescription>{t("add_event_description")}</DialogDescription>
          </DialogHeader>

          <Field>
            <FieldLabel htmlFor="event_name">
              {t("event_name_label")}
            </FieldLabel>

            <Input
              id="event_name"
              autoComplete="off"
              placeholder={t("event_name_placeholder")}
            />

            <FieldLabel className="mt-4 mb-2">
              {t("event_data_label")}
            </FieldLabel>

            <JsonBuilderField onChange={setEventData} />

            <Button
              onClick={async () => {
                const name = (
                  document.getElementById("event_name") as HTMLInputElement
                ).value;
                if (!name) return;
                console.log("name", name);
                await api.users.add_event(project.id, user.id, name, eventData as Record<string, unknown>);
                setAddEventModal(false);
                setRefreshKey((prev) => prev + 1);
              }}
            >
              {t("add_event")}
            </Button>
          </Field>
        </DialogContent>
      </Dialog>
      {event &&
        (hasPreview ? (
          <Modal
            title={event.name}
            size="fullscreen"
            open={event != null}
            onClose={() => setEvent(undefined)}
          >
            <Columns>
              <Column style={{ padding: "20px" }}>
                {formatDate(preferences, event.created_at)}
                <JsonPreview
                  value={{
                    name: event.name,
                    ...event.data,
                    created_at: event.created_at,
                  }}
                />
              </Column>
              <Column>
                {event.name === "email_sent" &&
                  event.data?.result?.message?.html && (
                    <Iframe
                      content={event.data.result.message.html ?? ""}
                      fullHeight={true}
                      width="100%"
                    />
                  )}
              </Column>
            </Columns>
          </Modal>
        ) : (
          <Modal
            title={event.name}
            description={formatDate(preferences, event.created_at)}
            size="large"
            open={event != null}
            onClose={() => setEvent(undefined)}
          >
            <JsonPreview
              value={{
                name: event.name,
                ...event.data,
                created_at: event.created_at,
              }}
            />
          </Modal>
        ))}
    </>
  );
}
