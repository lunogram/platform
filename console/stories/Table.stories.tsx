import type { Meta, StoryObj } from "@storybook/react-vite"
import { Users } from "lucide-react"
import type { ReactNode } from "react"
import { useState } from "react"

import { Badge } from "@/components/ui/badge"
import { Checkbox } from "@/components/ui/checkbox"
import { Skeleton } from "@/components/ui/skeleton"
import {
    Table,
    TableBody,
    TableCaption,
    TableCell,
    TableFooter,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"

const meta: Meta<typeof Table> = {
    title: "UI/Table",
    component: Table,
}

export default meta
type Story = StoryObj<typeof Table>

const users = [
    {
        id: "usr_01",
        name: "Jane Cooper",
        email: "jane@example.com",
        lists: 3,
        status: "Subscribed",
    },
    {
        id: "usr_02",
        name: "Wade Warren",
        email: "wade@example.com",
        lists: 1,
        status: "Subscribed",
    },
    {
        id: "usr_03",
        name: "Esther Howard",
        email: "esther@example.com",
        lists: 0,
        status: "Unsubscribed",
    },
    {
        id: "usr_04",
        name: "Cameron Williamson",
        email: "cameron@example.com",
        lists: 2,
        status: "Subscribed",
    },
]

function Header({ selectable }: { selectable?: ReactNode }) {
    return (
        <TableHeader>
            <TableRow>
                {selectable && <TableHead className="w-8">{selectable}</TableHead>}
                <TableHead>Name</TableHead>
                <TableHead>Email</TableHead>
                <TableHead className="text-right">Lists</TableHead>
                <TableHead>Status</TableHead>
            </TableRow>
        </TableHeader>
    )
}

export const Populated: Story = {
    render: () => (
        <div className="rounded-lg border bg-card">
            <Table>
                <TableCaption className="mb-4">Users matching “example.com”.</TableCaption>
                <Header />
                <TableBody>
                    {users.map((user) => (
                        <TableRow key={user.id}>
                            <TableCell className="font-medium">{user.name}</TableCell>
                            <TableCell className="text-muted-foreground">{user.email}</TableCell>
                            <TableCell className="text-right">{user.lists}</TableCell>
                            <TableCell>
                                <Badge
                                    variant={user.status === "Subscribed" ? "secondary" : "outline"}
                                >
                                    {user.status}
                                </Badge>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
                <TableFooter>
                    <TableRow>
                        <TableCell colSpan={2}>Total</TableCell>
                        <TableCell className="text-right">6</TableCell>
                        <TableCell />
                    </TableRow>
                </TableFooter>
            </Table>
        </div>
    ),
}

function SelectableTable() {
    const [selected, setSelected] = useState<string[]>(["usr_02"])
    const all = selected.length === users.length
    return (
        <div className="rounded-lg border bg-card">
            <Table>
                <Header
                    selectable={
                        <Checkbox
                            aria-label="Select all"
                            checked={all ? true : selected.length > 0 ? "indeterminate" : false}
                            onCheckedChange={() => setSelected(all ? [] : users.map((u) => u.id))}
                        />
                    }
                />
                <TableBody>
                    {users.map((user) => {
                        const isSelected = selected.includes(user.id)
                        return (
                            <TableRow
                                key={user.id}
                                data-state={isSelected ? "selected" : undefined}
                            >
                                <TableCell>
                                    <Checkbox
                                        aria-label={`Select ${user.name}`}
                                        checked={isSelected}
                                        onCheckedChange={() =>
                                            setSelected(
                                                isSelected
                                                    ? selected.filter((id) => id !== user.id)
                                                    : [...selected, user.id],
                                            )
                                        }
                                    />
                                </TableCell>
                                <TableCell className="font-medium">{user.name}</TableCell>
                                <TableCell className="text-muted-foreground">
                                    {user.email}
                                </TableCell>
                                <TableCell className="text-right">{user.lists}</TableCell>
                                <TableCell>{user.status}</TableCell>
                            </TableRow>
                        )
                    })}
                </TableBody>
            </Table>
        </div>
    )
}

export const Selectable: Story = {
    render: () => <SelectableTable />,
}

export const Empty: Story = {
    render: () => (
        <div className="rounded-lg border bg-card">
            <Table>
                <Header />
                <TableBody>
                    <TableRow>
                        <TableCell colSpan={4} className="h-32 text-center">
                            <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                <Users className="h-8 w-8" />
                                <p>No users yet</p>
                            </div>
                        </TableCell>
                    </TableRow>
                </TableBody>
            </Table>
        </div>
    ),
}

export const Loading: Story = {
    render: () => (
        <div className="rounded-lg border bg-card">
            <Table>
                <Header />
                <TableBody>
                    {Array.from({ length: 4 }).map((_, i) => (
                        <TableRow key={i}>
                            <TableCell>
                                <Skeleton className="h-4 w-32" />
                            </TableCell>
                            <TableCell>
                                <Skeleton className="h-4 w-40" />
                            </TableCell>
                            <TableCell>
                                <Skeleton className="ml-auto h-4 w-6" />
                            </TableCell>
                            <TableCell>
                                <Skeleton className="h-5 w-20" />
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </div>
    ),
}
