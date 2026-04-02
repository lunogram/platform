import type { Meta, StoryObj } from "@storybook/react"
import { useForm } from "react-hook-form"
import {
    Form,
    FormField,
    FormItem,
    FormLabel,
    FormControl,
    FormDescription,
    FormMessage,
} from "./form"
import { Input } from "./input"
import { Button } from "./button"

const meta: Meta = {
    title: "UI/Form",
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj

interface LoginFormValues {
    email: string
    password: string
}

export const LoginForm: Story = {
    render: () => {
        const form = useForm<LoginFormValues>({
            defaultValues: { email: "", password: "" },
        })

        const onSubmit = (values: LoginFormValues) => {
            console.log("Form submitted:", values)
        }

        return (
            <div className="w-[400px] p-6 rounded-lg border">
                <h2 className="text-lg font-semibold mb-6">Sign in</h2>
                <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                        <FormField
                            control={form.control}
                            name="email"
                            rules={{
                                required: "Email is required",
                                pattern: {
                                    value: /^[^\s@]+@[^\s@]+\.[^\s@]+$/,
                                    message: "Enter a valid email address",
                                },
                            }}
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>Email</FormLabel>
                                    <FormControl>
                                        <Input
                                            type="email"
                                            placeholder="you@example.com"
                                            {...field}
                                        />
                                    </FormControl>
                                    <FormDescription>
                                        We will never share your email.
                                    </FormDescription>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                        <FormField
                            control={form.control}
                            name="password"
                            rules={{
                                required: "Password is required",
                                minLength: {
                                    value: 8,
                                    message: "Password must be at least 8 characters",
                                },
                            }}
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>Password</FormLabel>
                                    <FormControl>
                                        <Input
                                            type="password"
                                            placeholder="••••••••"
                                            {...field}
                                        />
                                    </FormControl>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                        <Button type="submit" className="w-full">
                            Sign in
                        </Button>
                    </form>
                </Form>
            </div>
        )
    },
}

export const WithValidationErrors: Story = {
    render: () => {
        const form = useForm<LoginFormValues>({
            defaultValues: { email: "not-an-email", password: "123" },
            mode: "onChange",
        })

        // Trigger validation immediately
        const onSubmit = (values: LoginFormValues) => {
            console.log("Form submitted:", values)
        }

        return (
            <div className="w-[400px] p-6 rounded-lg border">
                <h2 className="text-lg font-semibold mb-6">Sign in — with errors</h2>
                <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                        <FormField
                            control={form.control}
                            name="email"
                            rules={{
                                required: "Email is required",
                                pattern: {
                                    value: /^[^\s@]+@[^\s@]+\.[^\s@]+$/,
                                    message: "Enter a valid email address",
                                },
                            }}
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>Email</FormLabel>
                                    <FormControl>
                                        <Input type="email" placeholder="you@example.com" {...field} />
                                    </FormControl>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                        <FormField
                            control={form.control}
                            name="password"
                            rules={{
                                required: "Password is required",
                                minLength: {
                                    value: 8,
                                    message: "Password must be at least 8 characters",
                                },
                            }}
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>Password</FormLabel>
                                    <FormControl>
                                        <Input type="password" placeholder="••••••••" {...field} />
                                    </FormControl>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                        <Button type="submit" className="w-full">
                            Sign in
                        </Button>
                    </form>
                </Form>
            </div>
        )
    },
}
