from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.scenario_source import ScenarioSource
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="Scenario")


@_attrs_define
class Scenario:
    id: UUID
    """ UUIDv7. """
    engagement_id: UUID
    ordinal: int
    """ 1-based dense position; UI order. """
    name: str
    narrative: str
    source: ScenarioSource
    """ Provenance of a scenario. `manual` is authored here; `ctid` and `imported` come from content. """
    created_at: datetime.datetime
    updated_at: datetime.datetime
    threat_actor: str | Unset = UNSET
    source_ref: str | Unset = UNSET
    """ External reference (CTID plan id, import key). """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        engagement_id = str(self.engagement_id)

        ordinal = self.ordinal

        name = self.name

        narrative = self.narrative

        source = self.source.value

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        threat_actor = self.threat_actor

        source_ref = self.source_ref

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "engagementId": engagement_id,
                "ordinal": ordinal,
                "name": name,
                "narrative": narrative,
                "source": source,
                "createdAt": created_at,
                "updatedAt": updated_at,
            }
        )
        if threat_actor is not UNSET:
            field_dict["threatActor"] = threat_actor
        if source_ref is not UNSET:
            field_dict["sourceRef"] = source_ref

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        engagement_id = UUID(d.pop("engagementId"))

        ordinal = d.pop("ordinal")

        name = d.pop("name")

        narrative = d.pop("narrative")

        source = ScenarioSource(d.pop("source"))

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        threat_actor = d.pop("threatActor", UNSET)

        source_ref = d.pop("sourceRef", UNSET)

        scenario = cls(
            id=id,
            engagement_id=engagement_id,
            ordinal=ordinal,
            name=name,
            narrative=narrative,
            source=source,
            created_at=created_at,
            updated_at=updated_at,
            threat_actor=threat_actor,
            source_ref=source_ref,
        )

        scenario.additional_properties = d
        return scenario

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
