import type { Meta, StoryObj } from "@storybook/react"
import {
    Table,
    TableHeader,
    TableBody,
    TableHead,
    TableRow,
    TableCell,
    TableCaption,
} from "./table"

const meta: Meta<typeof Table> = {
    component: Table,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Table>

const invoices = [
    { id: "INV-001", status: "Paid", method: "Credit Card", amount: "$250.00" },
    { id: "INV-002", status: "Pending", method: "PayPal", amount: "$150.00" },
    { id: "INV-003", status: "Unpaid", method: "Bank Transfer", amount: "$350.00" },
    { id: "INV-004", status: "Paid", method: "Credit Card", amount: "$450.00" },
]

export const Default: Story = {
    render: () => (
        <Table>
            <TableCaption>A list of your recent invoices.</TableCaption>
            <TableHeader>
                <TableRow>
                    <TableHead>Invoice</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Method</TableHead>
                    <TableHead style={{ textAlign: "right" }}>Amount</TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {invoices.map((invoice) => (
                    <TableRow key={invoice.id}>
                        <TableCell style={{ fontWeight: 500 }}>{invoice.id}</TableCell>
                        <TableCell>{invoice.status}</TableCell>
                        <TableCell>{invoice.method}</TableCell>
                        <TableCell style={{ textAlign: "right" }}>{invoice.amount}</TableCell>
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    ),
}

export const Simple: Story = {
    render: () => (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Status</TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                <TableRow>
                    <TableCell>Alice Johnson</TableCell>
                    <TableCell>Admin</TableCell>
                    <TableCell>Active</TableCell>
                </TableRow>
                <TableRow>
                    <TableCell>Bob Smith</TableCell>
                    <TableCell>Editor</TableCell>
                    <TableCell>Inactive</TableCell>
                </TableRow>
                <TableRow>
                    <TableCell>Carol White</TableCell>
                    <TableCell>Viewer</TableCell>
                    <TableCell>Active</TableCell>
                </TableRow>
            </TableBody>
        </Table>
    ),
}
