from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="ContentProcedureInputArg")


@_attrs_define
class ContentProcedureInputArg:
    """One parameterized input on a procedure template."""

    name: str
    """ Argument key as referenced in commands via `#{name}`. """
    description: str
    type_: str
    """ Upstream type label (`path`, `url`, `string`, `integer`, …). """
    default: str
    """ Default value coerced to string for a stable wire shape. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        description = self.description

        type_ = self.type_

        default = self.default

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
                "description": description,
                "type": type_,
                "default": default,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        name = d.pop("name")

        description = d.pop("description")

        type_ = d.pop("type")

        default = d.pop("default")

        content_procedure_input_arg = cls(
            name=name,
            description=description,
            type_=type_,
            default=default,
        )

        content_procedure_input_arg.additional_properties = d
        return content_procedure_input_arg

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
