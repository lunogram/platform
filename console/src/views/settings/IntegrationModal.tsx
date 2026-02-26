import { useContext, useEffect, useMemo, useState } from "react";
import { oapiClient } from "@/oapi/client";
import { ProjectContext } from "../../contexts";
import type {
  Project,
  Provider,
  ProviderCreateParams,
  ProviderMeta,
  ProviderUpdateParams,
} from "../../types";
import Alert from "../../ui/Alert";
import { Button } from "@/components/ui/button";
import SchemaFields from "../../ui/form/SchemaFields";
import TextInput from "../../ui/form/TextInput";
import RadioInput from "../../ui/form/RadioInput";
import FormWrapper from "../../ui/form/FormWrapper";
import type { ModalProps } from "../../ui/Modal";
import Modal from "../../ui/Modal";
import Tile, { TileGrid } from "../../ui/Tile";
import { snakeToTitle } from "../../utils";
import { ChevronLeftIcon } from "../../components/icons";
import { useTranslation } from "react-i18next";

import "./IntegrationModal.css";
import { t } from "i18next";

interface IntegrationFormParams {
  project: Project;
  meta: ProviderMeta;
  provider?: Provider;
  onChange: (provider: Provider) => void;
}

export function IntegrationForm({
  project,
  provider: defaultProvider,
  onChange,
  meta,
}: IntegrationFormParams) {
  const { t } = useTranslation();
  const [provider, setProvider] = useState<Provider | undefined>(
    defaultProvider,
  );
  // meta uses type/group, but API and Provider use module/channel
  const module = meta.type;
  const channel = meta.group;
  useEffect(() => {
    if (defaultProvider) {
      oapiClient.GET("/api/admin/projects/{projectID}/providers/{group}/{type}/{providerID}", {
        params: {
          path: {
            projectID: project.id,
            group: defaultProvider.channel,
            type: defaultProvider.module,
            providerID: defaultProvider.id,
          },
        },
      })
        .then((res) => {
          setProvider(res.data);
        })
        .catch(() => {});
    }
  }, [project.id, defaultProvider]);

  async function handleCreate({
    name,
    rate_limit,
    rate_interval,
    data = {},
  }: ProviderCreateParams | ProviderUpdateParams) {
    const params = { name, data, rate_limit, rate_interval };
    let value: Provider;

    if (provider?.id) {
      const res = await oapiClient.PATCH("/api/admin/projects/{projectID}/providers/{group}/{type}/{providerID}", {
        params: {
          path: {
            projectID: project.id,
            group: channel,
            type: module,
            providerID: provider.id,
          },
        },        
        body: params,
      });
      if (!res.data) {
        throw new Error("Failed to update provider");
      }
      value = res.data;
    } else {
      const res = await oapiClient.POST("/api/admin/projects/{projectID}/providers/{group}/{type}", {
        params: {
          path: {
            projectID: project.id,
            group: channel,
            type: module,
          },
        
        body: params,
        },
      });
      if (!res.data) {
        throw new Error("Failed to create provider");
      }
      value = res.data;
    }

    onChange(value);
  }

  return (
    <FormWrapper<ProviderCreateParams>
      onSubmit={async (provider) => await handleCreate(provider)}
      submitLabel={provider?.id ? "Update Integration" : "Create Integration"}
      defaultValues={provider}
    >
      {(form) => (
        <>
          {provider?.id ? (
            <>
              {provider.setup?.length > 0 && (
                <h4 className="legacy-typography">Details</h4>
              )}
              {provider.setup?.map((item) => {
                return (
                  <TextInput
                    name={item.name}
                    key={item.name}
                    value={item.value}
                    disabled
                  />
                );
              })}
            </>
          ) : (
            <Alert title={meta.name} variant="plain">
              Fill out the fields below to setup this integration. For more
              information on this integration please see the documentation on
              our website
            </Alert>
          )}

          <h4 className="legacy-typography">Config</h4>
          <TextInput.Field form={form} name="name" required />
          <SchemaFields
            parent="data"
            schema={meta.schema.properties.data}
            form={form}
          />
          <TextInput.Field
            form={form}
            type="number"
            name="rate_limit"
            subtitle="If you need to cap send rate, enter the maximum per interval limit."
          />
          <RadioInput.Field
            form={form}
            name="rate_interval"
            label={t("rate_interval")}
            options={[
              { key: "second", label: t("second") },
              { key: "minute", label: t("minute") },
              { key: "hour", label: t("hour") },
              { key: "day", label: t("day") },
            ]}
          />
        </>
      )}
    </FormWrapper>
  );
}

interface IntegrationModalProps extends Omit<ModalProps, "title"> {
  provider: Provider | undefined;
  onChange: (provider: Provider) => void;
}

export default function IntegrationModal({
  onChange,
  provider,
  ...props
}: IntegrationModalProps) {
  const [project] = useContext(ProjectContext);
  const [options, setOptions] = useState<ProviderMeta[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const fetchOptions = async () => {
      if (!project?.id) return;
      try {
        const res = await oapiClient.GET("/api/admin/projects/{projectID}/providers/meta", {
          params: {
            path: {
              projectID: project.id,
            },
          },
        });
        setOptions(res.data ?? []);
      } finally {
        setIsLoading(false);
      }
    };
    fetchOptions();
  }, [project?.id]);

  const [meta, setMeta] = useState<ProviderMeta | undefined>();

  const derivedMeta = useMemo(
    () =>
      options?.find(
        (item) =>
          item.group === provider?.channel && item.type === provider?.module,
      ),
    [options, provider],
  );

  const activeMeta = meta ?? derivedMeta;

  const handleChange = (provider: Provider) => {
    onChange(provider);
    props.onClose(false);
    setMeta(undefined);
  };

  if (provider?.external_id) {
    return (
      <Modal {...props} title={t("external_integration_title")} size="regular">
        <Alert title="Internal Integration" variant="plain">
          {t("external_integration_alert")}
        </Alert>
        <div style={{ marginTop: "20px" }}>
          <Button variant="secondary" onClick={() => props.onClose(false)}>
            <ChevronLeftIcon />
            {t("close")}
          </Button>
        </div>
      </Modal>
    );
  }

  return (
    <Modal
      {...props}
      title={
        activeMeta
          ? provider?.id
            ? `${provider?.name} (${activeMeta.name})`
            : "Setup Integration"
          : "Integrations"
      }
      size="regular"
    >
      {!activeMeta ? (
        <>
          <p>
            To get started, pick one of the integrations from the list below.
          </p>
          <TileGrid>
            {options?.map((option) => (
              <Tile
                key={`${option.group}${option.type}`}
                title={option.name}
                onClick={() => setMeta(option)}
                iconUrl={option.icon}
              >
                {snakeToTitle(option.group)}
              </Tile>
            ))}
          </TileGrid>
        </>
      ) : (
        <>
          {!provider?.id && (
            <div style={{ marginBottom: "10px" }}>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setMeta(undefined)}
              >
                <ChevronLeftIcon />
                Integrations
              </Button>
            </div>
          )}
          <IntegrationForm
            project={project}
            provider={provider}
            meta={activeMeta}
            onChange={handleChange}
          />
        </>
      )}
    </Modal>
  );
}
