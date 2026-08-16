from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.content_sync_job_kind import ContentSyncJobKind
from ..models.content_sync_job_status import ContentSyncJobStatus
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="ContentSyncJob")


@_attrs_define
class ContentSyncJob:
    """One content sync / reprocess / bundle / v1-import job."""

    id: UUID
    source_id: UUID
    kind: ContentSyncJobKind
    """ What a content sync job is doing. """
    status: ContentSyncJobStatus
    """ Lifecycle state of a content sync job. """
    phase: str
    """ Current pipeline phase: `fetch`, `parse`, `normalize`, `apply`,
    `finalize`, or empty before the worker picks the job up.
     """
    progress_current: int
    progress_total: int
    message: str
    error: str
    """ Failure detail when status is `failed`. Empty otherwise. """
    created_by: str
    """ User id that enqueued the job. Empty for system/blctl. """
    created_at: datetime.datetime
    version: str | Unset = UNSET
    """ Version token the job targeted or resolved, when known. """
    started_at: datetime.datetime | Unset = UNSET
    finished_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        source_id = str(self.source_id)

        kind = self.kind.value

        status = self.status.value

        phase = self.phase

        progress_current = self.progress_current

        progress_total = self.progress_total

        message = self.message

        error = self.error

        created_by = self.created_by

        created_at = self.created_at.isoformat()

        version = self.version

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        finished_at: str | Unset = UNSET
        if not isinstance(self.finished_at, Unset):
            finished_at = self.finished_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "sourceId": source_id,
                "kind": kind,
                "status": status,
                "phase": phase,
                "progressCurrent": progress_current,
                "progressTotal": progress_total,
                "message": message,
                "error": error,
                "createdBy": created_by,
                "createdAt": created_at,
            }
        )
        if version is not UNSET:
            field_dict["version"] = version
        if started_at is not UNSET:
            field_dict["startedAt"] = started_at
        if finished_at is not UNSET:
            field_dict["finishedAt"] = finished_at

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        source_id = UUID(d.pop("sourceId"))

        kind = ContentSyncJobKind(d.pop("kind"))

        status = ContentSyncJobStatus(d.pop("status"))

        phase = d.pop("phase")

        progress_current = d.pop("progressCurrent")

        progress_total = d.pop("progressTotal")

        message = d.pop("message")

        error = d.pop("error")

        created_by = d.pop("createdBy")

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        version = d.pop("version", UNSET)

        _started_at = d.pop("startedAt", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _finished_at = d.pop("finishedAt", UNSET)
        finished_at: datetime.datetime | Unset
        if isinstance(_finished_at, Unset):
            finished_at = UNSET
        else:
            finished_at = datetime.datetime.fromisoformat(_finished_at)

        content_sync_job = cls(
            id=id,
            source_id=source_id,
            kind=kind,
            status=status,
            phase=phase,
            progress_current=progress_current,
            progress_total=progress_total,
            message=message,
            error=error,
            created_by=created_by,
            created_at=created_at,
            version=version,
            started_at=started_at,
            finished_at=finished_at,
        )

        content_sync_job.additional_properties = d
        return content_sync_job

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
