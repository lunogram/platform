import { useState, useMemo } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area";
import { Search } from "lucide-react";
import type { User } from "@/types";

interface UserSelectionModalProps {
  users: User[];
  isOpen: boolean;
  onClose: () => void;
  onSelect: (user: User) => void;
}

/** 
 * Represents a column definition to safely access either
 * root properties or nested 'data' properties.
 */
interface ColumnDef {
  key: string;
  label: string;
  source: "root" | "data";
}

export function UserSelectionModal({
  users,
  isOpen,
  onClose,
  onSelect,
}: UserSelectionModalProps) {
  const [searchTerm, setSearchTerm] = useState("");

  // 1. Generate columns dynamically from the first user's structure
  const columns = useMemo<ColumnDef[]>(() => {
    if (users.length === 0) return [];

    const cols: ColumnDef[] = [
      { key: "id", label: "ID", source: "root" },
      { key: "full_name", label: "Full Name", source: "root" },
      { key: "email", label: "Email", source: "root" },
      { key: "timezone", label: "Timezone", source: "root" },
    ];

    // Dynamically add keys found in the 'data' object
    const dataKeys = Object.keys(users[0].data);
    dataKeys.forEach((key) => {
      // Avoid duplicating columns if they exist in root
      if (!cols.find((c) => c.key === key)) {
        cols.push({ key, label: key, source: "data" });
      }
    });

    return cols;
  }, [users]);

  // 2. Simple filter logic based on stringified user object
  const filteredUsers = useMemo(() => {
    return users.filter((user) => {
      const searchableString = JSON.stringify(user).toLowerCase();
      return searchableString.includes(searchTerm.toLowerCase());
    });
  }, [users, searchTerm]);

  // Helper to extract value safely without 'any'
  const getCellValue = (user: User, col: ColumnDef): string => {
    if (col.source === "root") {
      const value = user[col.key as keyof User];
      return value ? String(value) : "-";
    }
    const dataValue = user.data[col.key];
    return dataValue !== undefined && dataValue !== null ? String(dataValue) : "-";
  };

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-5xl w-[95vw] h-[80vh] flex flex-col p-0 overflow-hidden">
        <DialogHeader className="p-6 pb-2">
          <DialogTitle>Select User</DialogTitle>
          <DialogDescription>
            Choose a user to initiate the journey. Search by name, ID, or attributes.
          </DialogDescription>
          
          <div className="relative mt-4">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Filter users..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="pl-10"
            />
          </div>
        </DialogHeader>

        <div className="flex-1 min-h-0 border-t mt-2">
          <ScrollArea className="h-full">
            <div className="p-6 pt-0">
              <Table>
                <TableHeader className="bg-background sticky top-0 z-10">
                  <TableRow>
                    {columns.map((col) => (
                      <TableHead key={`${col.source}-${col.key}`} className="capitalize whitespace-nowrap">
                        {col.label.replace("_", " ")}
                      </TableHead>
                    ))}
                    <TableHead className="text-right">Action</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredUsers.length > 0 ? (
                    filteredUsers.map((user) => (
                      <TableRow key={user.id} className="group">
                        {columns.map((col) => (
                          <TableCell key={`${col.source}-${col.key}`} className="max-w-[200px] truncate">
                            {getCellValue(user, col)}
                          </TableCell>
                        ))}
                        <TableCell className="text-right">
                          <Button 
                            variant="secondary" 
                            size="sm"
                            onClick={() => onSelect(user)}
                          >
                            Select
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))
                  ) : (
                    <TableRow>
                      <TableCell colSpan={columns.length + 1} className="h-32 text-center">
                        No users found.
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>
            <ScrollBar orientation="horizontal" />
          </ScrollArea>
        </div>
      </DialogContent>
    </Dialog>
  );
}