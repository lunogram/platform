import { useParams, useSearchParams } from "react-router";
import { useEffect, useState } from "react";
import { useClerk } from "@clerk/clerk-react";
import { oapiClient } from "@/oapi/client";
import { AUTH_DRIVERS } from "../../types";
import { Alert } from "../../ui";
import { useTranslation } from "react-i18next";

import "./Auth.css";

export default function LoginCallback() {
  const { t } = useTranslation();
  const { session } = useClerk();
  const { driver } = useParams() as { driver: string };
  const [searchParams] = useSearchParams();
  const [error, setError] = useState<string>();
  const redirect = searchParams.get("r") ?? "/";

  useEffect(() => {
    const handleAuth = async () => {
      try {
        switch (driver) {
          case AUTH_DRIVERS.CLERK: {
            if (!session) return;

            // Get the session token from Clerk
            const token = await session.getToken();
            if (!token) {
              setError(t("login_callback_error"));
              return;
            }

            await oapiClient.POST("/api/auth/login/{driver}/callback", {
              params: { path: { driver: "clerk" } },
              body: { redirect },
            });
            break;
          }
          default:
            setError(t("login_unsupported_driver"));
            return;
        }

        // Redirect on success
        window.location.href = redirect;
      } catch (err) {
        console.error("Auth callback error:", err);
        setError(t("login_callback_error"));
      }
    };

    handleAuth();
  }, [driver, redirect, session, t]);

  if (error) {
    return (
      <div className="auth login">
        <div className="auth-step">
          <Alert variant="destructive" title={t("error")}>
            {error}
          </Alert>
        </div>
      </div>
    );
  }

  return (
    <div className="auth login">
      <div className="auth-step">
        <p>{t("authenticating")}</p>
      </div>
    </div>
  );
}
