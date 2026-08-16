from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="PublishReport")


@_attrs_define
class PublishReport:
    include_evidence: bool | Unset = False
    """ Include evidence bytes in the published version. """

    def to_dict(self) -> dict[str, Any]:
        include_evidence = self.include_evidence

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if include_evidence is not UNSET:
            field_dict["includeEvidence"] = include_evidence

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        include_evidence = d.pop("includeEvidence", UNSET)

        publish_report = cls(
            include_evidence=include_evidence,
        )

        return publish_report
