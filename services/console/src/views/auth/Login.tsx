import { useSearchParams } from "react-router";
import { SignIn } from "@clerk/clerk-react";
import { useEffect, useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Loader2, Mail, Lock, ArrowLeft } from "lucide-react";
import { useForm } from "react-hook-form";

import api from "../../api";
import { type AuthDriver, AUTH_DRIVERS } from "../../types";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";

interface LoginFormValues {
  email: string;
  password: string;
}

const SUPPORTED_DRIVERS: AuthDriver[] = [
  AUTH_DRIVERS.BASIC,
  AUTH_DRIVERS.CLERK,
];

export default function Login() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [drivers, setDrivers] = useState<AuthDriver[]>();
  const [selectedDriver, setSelectedDriver] = useState<AuthDriver>();
  const [error, setError] = useState<string>();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const redirect = searchParams.get("r") ?? "/";

  const form = useForm<LoginFormValues>({
    defaultValues: {
      email: "",
      password: "",
    },
  });

  const handleSelectDriver = useCallback((driver: AuthDriver) => {
    setSelectedDriver(driver);
    setError(undefined);
  }, []);

  const handleBasicAuth = async (data: LoginFormValues) => {
    if (!data.password) {
      setError(t("login_basic_instructions"));
      return;
    }

    setIsSubmitting(true);
    try {
      await api.auth.basicAuth(
        data.email,
        data.password,
        searchParams.get("r") ?? "/",
      );
    } catch (err) {
      console.error("Basic auth failed:", err);
      setError(t("login_invalid_credentials"));
    } finally {
      setIsSubmitting(false);
    }
  };

  useEffect(() => {
    api.auth
      .methods()
      .then((methods) => {
        const supportedDrivers = methods.filter((driver) =>
          SUPPORTED_DRIVERS.includes(driver),
        );
        setDrivers(supportedDrivers);
        if (supportedDrivers.length === 1) {
          handleSelectDriver(supportedDrivers[0]);
        }
      })
      .catch((err) => {
        console.error("Failed to fetch auth methods:", err);
        setError(t("login_methods_error"));
      });
  }, [handleSelectDriver, t]);

  // Loading state
  if (!drivers || drivers.length === 0) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-muted/40">
        <Card className="w-full max-w-sm">
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            <p className="mt-4 text-sm text-muted-foreground">{t("loading")}</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted/40 p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1 text-center">
          <CardTitle className="text-2xl font-bold">{t("welcome")}</CardTitle>
          {!selectedDriver && (
            <CardDescription>{t("login_select_method")}</CardDescription>
          )}
          {selectedDriver === "basic" && (
            <CardDescription>{t("login_basic_instructions")}</CardDescription>
          )}
        </CardHeader>

        <CardContent className="space-y-4">
          {error && (
            <Alert variant="destructive">
              <AlertTitle>{t("error")}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {/* Driver selection */}
          {!selectedDriver && (
            <div className="flex flex-col gap-3">
              {drivers.map((driver) => (
                <Button
                  key={driver}
                  variant="outline"
                  className="w-full"
                  onClick={() => handleSelectDriver(driver)}
                >
                  {t(`auth_driver_${driver}`)}
                </Button>
              ))}
            </div>
          )}

          {/* Basic auth - email/password form */}
          {selectedDriver === AUTH_DRIVERS.BASIC && (
            <Form {...form}>
              <form
                onSubmit={form.handleSubmit(handleBasicAuth)}
                className="space-y-4"
              >
                <FormField
                  control={form.control}
                  name="email"
                  rules={{ required: t("field_required") }}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("email")}</FormLabel>
                      <FormControl>
                        <div className="relative">
                          <Mail className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                          <Input
                            type="email"
                            placeholder="name@example.com"
                            className="pl-9"
                            {...field}
                          />
                        </div>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="password"
                  rules={{ required: t("field_required") }}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("password")}</FormLabel>
                      <FormControl>
                        <div className="relative">
                          <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                          <Input type="password" className="pl-9" {...field} />
                        </div>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <Button
                  type="submit"
                  className="w-full"
                  disabled={isSubmitting}
                >
                  {isSubmitting && (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  )}
                  {t("submit")}
                </Button>

                {drivers.length > 1 && (
                  <Button
                    type="button"
                    variant="ghost"
                    className="w-full"
                    onClick={() => setSelectedDriver(undefined)}
                  >
                    <ArrowLeft className="mr-2 h-4 w-4" />
                    {t("back")}
                  </Button>
                )}
              </form>
            </Form>
          )}

          {/* Clerk auth - Clerk's SignIn component */}
          {selectedDriver === AUTH_DRIVERS.CLERK && (
            <div className="space-y-4">
              <SignIn
                forceRedirectUrl={`/login/clerk/callback?r=${redirect}`}
              />
              {drivers.length > 1 && (
                <Button
                  variant="ghost"
                  className="w-full"
                  onClick={() => setSelectedDriver(undefined)}
                >
                  <ArrowLeft className="mr-2 h-4 w-4" />
                  {t("back")}
                </Button>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
