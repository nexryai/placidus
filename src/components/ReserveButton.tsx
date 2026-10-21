import { IconCirclePlus, IconLoader2 } from "@tabler/icons-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { mutate } from "../api";
import { Button } from "./Ripple";
import { Notice } from "./Notice";

export function ReserveButton({
  programID,
  children = "Reserve",
  small = false,
}: {
  programID: number;
  children?: string;
  small?: boolean;
}) {
  const navigate = useNavigate();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  async function reserve() {
    setPending(true);
    setError("");
    try {
      const result = await mutate("/api/reservations", programID);
      navigate("/reservations", { state: { message: result.message } });
    } catch (error) {
      setError((error as Error).message);
    } finally {
      setPending(false);
    }
  }
  return (
    <>
      {error && <Notice kind="error">{error}</Notice>}
      <Button
        className={small ? "small" : ""}
        disabled={pending}
        onClick={reserve}
      >
        {pending ? (
          <IconLoader2 size={16} aria-hidden="true" />
        ) : (
          <IconCirclePlus size={16} aria-hidden="true" />
        )}
        {pending ? "Reserving…" : children}
      </Button>
    </>
  );
}
