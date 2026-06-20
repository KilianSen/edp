import LogStream from "./LogStream";

// LogDrawer is a slide-in panel that live-tails one deployment's log. Used from
// the dashboard for a quick look without leaving the page.
export default function LogDrawer({
  instanceID,
  deploymentID,
  title,
  onClose,
  onDone,
}: {
  instanceID: number;
  deploymentID: number;
  title: string;
  onClose: () => void;
  onDone?: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex" onClick={onClose}>
      <div className="absolute inset-0 bg-black/50" />
      <div
        className="relative ml-auto flex h-full w-full max-w-2xl flex-col border-l border-[#262e42] bg-[#0c111c] p-5"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center gap-3">
          <span className="font-medium">{title}</span>
          <button
            onClick={onClose}
            className="ml-auto rounded-md border border-[#33405d] px-3 py-1 text-sm text-[#8595b6] hover:text-[#e7ebf4]"
          >
            Close
          </button>
        </div>
        <LogStream instanceID={instanceID} deploymentID={deploymentID} heightClass="grow" onDone={() => onDone?.()} />
      </div>
    </div>
  );
}
