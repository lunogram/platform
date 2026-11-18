import type { Meta, StoryObj } from "@storybook/react-vite"
import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect } from "react"
import { useForm } from "react-hook-form"
import * as z from "zod"

import { Button } from "@/components/ui/button"
import {
    Form,
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"

const meta: Meta = {
    title: "UI/Form",
    parameters: {
        docs: {
            description: {
                component:
                    "react-hook-form with a zod schema, as in `src/validation/`. Labels, controls, and messages pick up the field's error state.",
            },
        },
    },
}

export default meta
type Story = StoryObj

const schema = z.object({
    name: z.string().min(1, "Name is required"),
    email: z.email("Enter a valid email address"),
    description: z.string().max(80, "Keep the description under 80 characters"),
})

type Values = z.infer<typeof schema>

function ProjectForm({ defaultValues, validate }: { defaultValues: Values; validate?: boolean }) {
    const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues })

    useEffect(() => {
        if (validate) void form.trigger()
    }, [form, validate])

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(() => {})} className="max-w-md space-y-6">
                <FormField
                    control={form.control}
                    name="name"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>Name</FormLabel>
                            <FormControl>
                                <Input placeholder="Northwind Mobile" {...field} />
                            </FormControl>
                            <FormDescription>Shown in the workspace switcher.</FormDescription>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <FormField
                    control={form.control}
                    name="email"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>Support email</FormLabel>
                            <FormControl>
                                <Input type="email" placeholder="support@example.com" {...field} />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <FormField
                    control={form.control}
                    name="description"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>Description</FormLabel>
                            <FormControl>
                                <Textarea {...field} />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <div className="flex justify-end gap-2">
                    <Button type="button" variant="outline" onClick={() => form.reset()}>
                        Reset
                    </Button>
                    <Button type="submit">Save</Button>
                </div>
            </form>
        </Form>
    )
}

export const Valid: Story = {
    render: () => (
        <ProjectForm
            defaultValues={{
                name: "Northwind Mobile",
                email: "support@northwind.example",
                description: "Customer messaging for the mobile app.",
            }}
        />
    ),
}

export const WithErrors: Story = {
    name: "With errors",
    render: () => (
        <ProjectForm
            validate
            defaultValues={{
                name: "",
                email: "support@",
                description:
                    "A description that runs far past the limit so the length validation shows its message.",
            }}
        />
    ),
}
