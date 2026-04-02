import type { Meta, StoryObj } from "@storybook/react"
import { DataTable } from "./data-table"
import type { DataTableCol } from "./data-table"
import { PreferencesProvider } from "@/contexts/PreferencesContext"

interface User {
    id: string
    name: string
    email: string
    role: string
    active: boolean
}

const columns: DataTableCol<User>[] = [
    { key: "id", title: "ID" },
    { key: "name", title: "Name" },
    { key: "email", title: "Email" },
    { key: "role", title: "Role" },
    { key: "active", title: "Active" },
]

const sampleUsers: User[] = [
    { id: "usr_001", name: "Alice Smith", email: "alice@example.com", role: "Admin", active: true },
    { id: "usr_002", name: "Bob Jones", email: "bob@example.com", role: "Editor", active: true },
    {
        id: "usr_003",
        name: "Carol White",
        email: "carol@example.com",
        role: "Viewer",
        active: false,
    },
    {
        id: "usr_004",
        name: "David Brown",
        email: "david@example.com",
        role: "Editor",
        active: true,
    },
]

const meta: Meta<typeof DataTable> = {
    title: "Components/DataTable",
    component: DataTable,
    tags: ["autodocs"],
    decorators: [
        (Story) => (
            <PreferencesProvider>
                <Story />
            </PreferencesProvider>
        ),
    ],
}

export default meta
type Story = StoryObj<typeof DataTable>

export const Default: Story = {
    render: () => (
        <DataTable<User> columns={columns} items={sampleUsers} />
    ),
}

export const Loading: Story = {
    render: () => (
        <DataTable<User> columns={columns} isLoading />
    ),
}

export const Empty: Story = {
    render: () => (
        <DataTable<User>
            columns={columns}
            items={[]}
            emptyMessage="No users found. Add a user to get started."
        />
    ),
}

export const Sortable: Story = {
    render: () => (
        <DataTable<User>
            columns={[
                { key: "id", title: "ID" },
                { key: "name", title: "Name", sortable: true },
                { key: "email", title: "Email", sortable: true },
                { key: "role", title: "Role" },
                { key: "active", title: "Active" },
            ]}
            items={sampleUsers}
        />
    ),
}
