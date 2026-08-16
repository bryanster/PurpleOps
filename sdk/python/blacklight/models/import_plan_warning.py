from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="ImportPlanWarning")


@_attrs_define
class ImportPlanWarning:
    step_ordinal: int
    """ The 1-based ordinal of the imported step within the scenario. """
    step_name: str
    """ Name of the imported step from the catalog. """
    technique_external_id: str
    """ The technique external id that did not resolve. """
    message: str
    """ Human-readable explanation. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        step_ordinal = self.step_ordinal

        step_name = self.step_name

        technique_external_id = self.technique_external_id

        message = self.message

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "stepOrdinal": step_ordinal,
                "stepName": step_name,
                "techniqueExternalId": technique_external_id,
                "message": message,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        step_ordinal = d.pop("stepOrdinal")

        step_name = d.pop("stepName")

        technique_external_id = d.pop("techniqueExternalId")

        message = d.pop("message")

        import_plan_warning = cls(
            step_ordinal=step_ordinal,
            step_name=step_name,
            technique_external_id=technique_external_id,
            message=message,
        )

        import_plan_warning.additional_properties = d
        return import_plan_warning

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
