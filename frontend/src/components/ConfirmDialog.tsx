interface Props {
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({
  title,
  message,
  confirmText = '确认',
  cancelText = '取消',
  busy,
  onConfirm,
  onCancel,
}: Props) {
  return (
    <div className="confirm-overlay" onClick={onCancel}>
      <div className="confirm-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="confirm-title">{title}</div>
        <div className="confirm-message">{message}</div>
        <div className="confirm-actions">
          <button className="confirm-btn confirm-btn--cancel" onClick={onCancel} disabled={busy}>
            {cancelText}
          </button>
          <button className="confirm-btn confirm-btn--confirm" onClick={onConfirm} disabled={busy}>
            {busy ? '…' : confirmText}
          </button>
        </div>
      </div>
    </div>
  )
}
