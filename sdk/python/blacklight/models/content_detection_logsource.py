from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="ContentDetectionLogsource")


@_attrs_define
class ContentDetectionLogsource:
    """Structured logsource fields for a custom detection rule. Known keys
    only — request bodies must not accept free-form properties
    (PLAN.md §4). Upstream Sigma rules still store arbitrary logsource
    JSON on the read model.

    """

    category: str | Unset = UNSET
    product: str | Unset = UNSET
    service: str | Unset = UNSET
    definition: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        category = self.category

        product = self.product

        service = self.service

        definition = self.definition

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if category is not UNSET:
            field_dict["category"] = category
        if product is not UNSET:
            field_dict["product"] = product
        if service is not UNSET:
            field_dict["service"] = service
        if definition is not UNSET:
            field_dict["definition"] = definition

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        category = d.pop("category", UNSET)

        product = d.pop("product", UNSET)

        service = d.pop("service", UNSET)

        definition = d.pop("definition", UNSET)

        content_detection_logsource = cls(
            category=category,
            product=product,
            service=service,
            definition=definition,
        )

        return content_detection_logsource
