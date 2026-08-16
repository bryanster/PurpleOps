from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from uuid import UUID


T = TypeVar("T", bound="ImportPlanRequest")


@_attrs_define
class ImportPlanRequest:
    """Request to import a CTID emulation plan as an engagement Scenario.
    At least one of `planId` or (`planExternalId` + `sourceId`) must be
    provided. If both are given `planId` takes precedence.

    """

    plan_id: UUID | Unset = UNSET
    """ Content catalog surrogate id of the plan to import. """
    plan_external_id: str | Unset = UNSET
    """ External id of the plan (e.g. CTID upstream id or actor slug). """
    source_id: UUID | Unset = UNSET
    """ Content source id. Required when `planExternalId` is used. """
    name: str | Unset = UNSET
    """ Override the scenario name (defaults to plan name). """
    starting_ordinal: int | Unset = UNSET
    """ Hint for the starting ordinal of the first imported step. Defaults to 1. Subsequent steps follow
    sequentially. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        plan_id: str | Unset = UNSET
        if not isinstance(self.plan_id, Unset):
            plan_id = str(self.plan_id)

        plan_external_id = self.plan_external_id

        source_id: str | Unset = UNSET
        if not isinstance(self.source_id, Unset):
            source_id = str(self.source_id)

        name = self.name

        starting_ordinal = self.starting_ordinal

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if plan_id is not UNSET:
            field_dict["planId"] = plan_id
        if plan_external_id is not UNSET:
            field_dict["planExternalId"] = plan_external_id
        if source_id is not UNSET:
            field_dict["sourceId"] = source_id
        if name is not UNSET:
            field_dict["name"] = name
        if starting_ordinal is not UNSET:
            field_dict["startingOrdinal"] = starting_ordinal

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        _plan_id = d.pop("planId", UNSET)
        plan_id: UUID | Unset
        if isinstance(_plan_id, Unset):
            plan_id = UNSET
        else:
            plan_id = UUID(_plan_id)

        plan_external_id = d.pop("planExternalId", UNSET)

        _source_id = d.pop("sourceId", UNSET)
        source_id: UUID | Unset
        if isinstance(_source_id, Unset):
            source_id = UNSET
        else:
            source_id = UUID(_source_id)

        name = d.pop("name", UNSET)

        starting_ordinal = d.pop("startingOrdinal", UNSET)

        import_plan_request = cls(
            plan_id=plan_id,
            plan_external_id=plan_external_id,
            source_id=source_id,
            name=name,
            starting_ordinal=starting_ordinal,
        )

        import_plan_request.additional_properties = d
        return import_plan_request

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
