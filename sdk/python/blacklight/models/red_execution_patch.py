from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.execution_status import ExecutionStatus
from ..types import UNSET, Unset
from typing import cast
import datetime


T = TypeVar("T", bound="RedExecutionPatch")


@_attrs_define
class RedExecutionPatch:
    """Red-side only PATCH body for an execution. `version` is the
    optimistic-lock field and is required on every call. Detection
    fields are not present — blue writes through a separate endpoint
    (M3-007) with its own type.

    """

    version: int
    """ The version the caller read. Mismatch → 409. """
    status: ExecutionStatus | Unset = UNSET
    """ The red-side state of one execution. """
    started_at: datetime.datetime | Unset = UNSET
    """ When execution began. If omitted on a →running transition, the server sets it to UTC now. """
    ended_at: datetime.datetime | Unset = UNSET
    command_run: str | Unset = UNSET
    source_host: str | Unset = UNSET
    target_host: str | Unset = UNSET
    red_notes: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        version = self.version

        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        ended_at: str | Unset = UNSET
        if not isinstance(self.ended_at, Unset):
            ended_at = self.ended_at.isoformat()

        command_run = self.command_run

        source_host = self.source_host

        target_host = self.target_host

        red_notes = self.red_notes

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "version": version,
            }
        )
        if status is not UNSET:
            field_dict["status"] = status
        if started_at is not UNSET:
            field_dict["startedAt"] = started_at
        if ended_at is not UNSET:
            field_dict["endedAt"] = ended_at
        if command_run is not UNSET:
            field_dict["commandRun"] = command_run
        if source_host is not UNSET:
            field_dict["sourceHost"] = source_host
        if target_host is not UNSET:
            field_dict["targetHost"] = target_host
        if red_notes is not UNSET:
            field_dict["redNotes"] = red_notes

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        version = d.pop("version")

        _status = d.pop("status", UNSET)
        status: ExecutionStatus | Unset
        if isinstance(_status, Unset):
            status = UNSET
        else:
            status = ExecutionStatus(_status)

        _started_at = d.pop("startedAt", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _ended_at = d.pop("endedAt", UNSET)
        ended_at: datetime.datetime | Unset
        if isinstance(_ended_at, Unset):
            ended_at = UNSET
        else:
            ended_at = datetime.datetime.fromisoformat(_ended_at)

        command_run = d.pop("commandRun", UNSET)

        source_host = d.pop("sourceHost", UNSET)

        target_host = d.pop("targetHost", UNSET)

        red_notes = d.pop("redNotes", UNSET)

        red_execution_patch = cls(
            version=version,
            status=status,
            started_at=started_at,
            ended_at=ended_at,
            command_run=command_run,
            source_host=source_host,
            target_host=target_host,
            red_notes=red_notes,
        )

        red_execution_patch.additional_properties = d
        return red_execution_patch

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
