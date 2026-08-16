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


T = TypeVar("T", bound="ContentSyncJobSummary")


@_attrs_define
class ContentSyncJobSummary:
    """The most recent job for a source, as detail surfaces it. Prefer
    `ContentSyncJob` when reading a job by id.

    """

    id: UUID
    kind: ContentSyncJobKind
    """ What a content sync job is doing. """
    status: ContentSyncJobStatus
    """ Lifecycle state of a content sync job. """
    created_at: datetime.datetime
    version: str | Unset = UNSET
    """ Version token the job targeted, when one was named. """
    phase: str | Unset = UNSET
    message: str | Unset = UNSET
    error: str | Unset = UNSET
    started_at: datetime.datetime | Unset = UNSET
    finished_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        kind = self.kind.value

        status = self.status.value

        created_at = self.created_at.isoformat()

        version = self.version

        phase = self.phase

        message = self.message

        error = self.error

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
                "kind": kind,
                "status": status,
                "createdAt": created_at,
            }
        )
        if version is not UNSET:
            field_dict["version"] = version
        if phase is not UNSET:
            field_dict["phase"] = phase
        if message is not UNSET:
            field_dict["message"] = message
        if error is not UNSET:
            field_dict["error"] = error
        if started_at is not UNSET:
            field_dict["startedAt"] = started_at
        if finished_at is not UNSET:
            field_dict["finishedAt"] = finished_at

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        kind = ContentSyncJobKind(d.pop("kind"))

        status = ContentSyncJobStatus(d.pop("status"))

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        version = d.pop("version", UNSET)

        phase = d.pop("phase", UNSET)

        message = d.pop("message", UNSET)

        error = d.pop("error", UNSET)

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

        content_sync_job_summary = cls(
            id=id,
            kind=kind,
            status=status,
            created_at=created_at,
            version=version,
            phase=phase,
            message=message,
            error=error,
            started_at=started_at,
            finished_at=finished_at,
        )

        content_sync_job_summary.additional_properties = d
        return content_sync_job_summary

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
